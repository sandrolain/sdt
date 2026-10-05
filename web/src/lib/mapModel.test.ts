import { describe, expect, it, vi } from "vitest";
import { layoutMap } from "./mapLayout";
import { buildMapGraph, mapNodeData, nodeRects, toCanvasDocument, visibleNodes } from "./mapModel";
import { measureTree, type MeasuredNode } from "./mapMetrics";
import { parseMapDocument } from "./mindmap";

function graph(
  md: string,
  options: Parameters<typeof buildMapGraph>[2] = {},
  kind: "balanced" | "radial" = "balanced",
) {
  const tree = measureTree(parseMapDocument(md));
  const { positions } = layoutMap(tree, kind);
  return { tree, ...buildMapGraph(tree, positions, { ...options, layout: kind }) };
}

describe("visibleNodes", () => {
  it("hides the descendants of a collapsed node and keeps the node itself", () => {
    const tree = measureTree(parseMapDocument("# Root\n\n- a\n  - a1\n  - a2\n- b\n"));
    const visible = visibleNodes(tree, new Set(["n0.1"]));
    expect(visible.map((n) => n.id)).toEqual(["n0", "n0.1", "n0.2"]);
  });

  it("hides a whole collapsed subtree", () => {
    const tree = measureTree(parseMapDocument("# Root\n\n- a\n  - a1\n    - a1a\n- b\n"));
    expect(visibleNodes(tree, new Set(["n0.1"])).map((n) => n.id)).toEqual(["n0", "n0.1", "n0.2"]);
  });
});

describe("buildMapGraph", () => {
  it("maps every node with its computed box and a top-left position", () => {
    const { tree, nodes } = graph("# Root\n\n- child\n");
    const root = nodes[0];
    expect(root.id).toBe("n0");
    expect(root.position).toEqual({ x: -tree.box.width / 2, y: -tree.box.height / 2 });
    expect(root.width).toBe(tree.box.width);
    expect(root.height).toBe(tree.box.height);
    expect(nodes.map((n) => n.id)).toEqual(["n0", "n0.1"]);
  });

  it("links parent to child and drops the links into a collapsed branch", () => {
    const { nodes, edges } = graph("# Root\n\n- a\n  - a1\n- b\n", {
      collapsed: new Set(["n0.1"]),
    });
    // The collapsed node keeps its own edge; only its descendants lose theirs.
    expect(edges.map((e) => e.id)).toEqual(["e:n0->n0.1", "e:n0->n0.2"]);
    expect(nodes.map((n) => n.id)).toEqual(["n0", "n0.1", "n0.2"]);
  });

  it("draws one labelled edge per paired relationship", () => {
    const { edges } = graph("# Root\n\n- a [1]\n- b [^1](Cool)\n");
    const relation = edges.find((e) => e.id === "r:1");
    expect(relation).toMatchObject({ fromNode: "n0.1", toNode: "n0.2", label: "Cool" });
    expect(edges).toHaveLength(3);
  });

  it("drops an unpaired relationship end rather than drawing a half edge", () => {
    const { edges } = graph("# Root\n\n- a [9]\n- b\n");
    expect(edges.some((e) => e.id.startsWith("r:"))).toBe(false);
  });

  it("treats a whole-label link as an openable node and a mixed label as text", () => {
    const { nodes } = graph(
      "# Root\n\n- [Other](/wiki/other.map)\n- see [Other](/wiki/other.map)\n",
    );
    expect(nodes[1].data.href).toBe("/wiki/other.map");
    expect(nodes[2].data.href).toBeUndefined();
    expect(nodes[2].data.text).toBe("see Other");
  });

  it("carries the markers the node and the interactions need", () => {
    const { nodes } = graph("# Root\n\n- topic [N:remember][!star][F]\n");
    expect(nodes[1].data.notes).toEqual(["remember"]);
    expect(nodes[1].data.stickers).toEqual(["star"]);
    expect(nodes[1].data.folded).toBe(true);
    expect(nodes[1].data.hasChildren).toBe(false);
  });

  it("passes the callbacks to every node and records the collapsed state", () => {
    const onToggle = vi.fn();
    const onOpen = vi.fn();
    const { nodes } = graph("# Root\n\n- a\n  - a1\n", { onToggle, onOpen });
    expect(nodes[1].data.onToggle).toBe(onToggle);
    expect(nodes[2].data.onOpen).toBe(onOpen);
    const collapsed = graph("# Root\n\n- a\n  - a1\n", { collapsed: new Set(["n0.1"]) });
    expect(collapsed.nodes[1].data.collapsed).toBe(true);
  });

  it("lays out the same graph in both layouts", () => {
    const md = "# Root\n\n- a\n- b\n";
    const balanced = graph(md, {}, "balanced");
    const radial = graph(md, {}, "radial");
    expect(balanced.nodes.map((n) => n.id)).toEqual(radial.nodes.map((n) => n.id));
    expect(balanced.nodes[1].position.x).not.toBe(radial.nodes[1].position.x);
  });
});

describe("nodeRects", () => {
  it("reads the computed boxes in layout coordinates", () => {
    const tree = measureTree(parseMapDocument("# Root\n\n- a\n"));
    const { nodes } = graph("# Root\n\n- a\n");
    const rects = nodeRects(nodes);
    expect(rects.get("n0")).toEqual({
      x: -tree.box.width / 2,
      y: -tree.box.height / 2,
      width: tree.box.width,
      height: tree.box.height,
    });
    expect(rects.size).toBe(nodes.length);
  });
});

describe("toCanvasDocument", () => {
  it("emits a JSON Canvas document whose nodes carry the x-map payload", () => {
    const { nodes, edges } = graph("# Root\n\n- child [!star]\n");
    const doc = toCanvasDocument({ nodes, edges });
    expect(doc.nodes[0]).toMatchObject({
      id: "n0",
      type: "text",
      x: nodes[0].position.x,
      y: nodes[0].position.y,
      width: nodes[0].width,
      height: nodes[0].height,
      label: "Root",
    });
    expect(mapNodeData(doc.nodes[1])?.stickers).toEqual(["star"]);
    expect(doc.edges).toBe(edges);
  });
});

/** A tree measured by hand, to check the model without the parser. */
const HAND: MeasuredNode = measureTree({
  content: "Root",
  children: [{ content: "child", children: [], payload: { kind: "topic" } }],
  payload: { kind: "topic" },
});

describe("buildMapGraph on a hand-built tree", () => {
  it("keeps the ids the metrics assigned", () => {
    const { nodes } = buildMapGraph(
      HAND,
      new Map([
        ["n0", { x: 0, y: 0 }],
        ["n0.1", { x: 10, y: 10 }],
      ]),
    );
    expect(nodes.map((n) => n.id)).toEqual(["n0", "n0.1"]);
  });
});

describe("buildMapGraph edge sides", () => {
  it("carries layout-emitted sides on parent edges, none on relation edges", () => {
    const { edges } = graph("# Root\n\n- a\n- b [1]\n- c [^1](Cool)\n");
    const eA = edges.find((e) => e.id === "e:n0->n0.1");
    const eB = edges.find((e) => e.id === "e:n0->n0.2");
    expect(eA?.fromSide).toBe("left");
    expect(eA?.toSide).toBe("right");
    expect(eB?.fromSide).toBe("right");
    expect(eB?.toSide).toBe("left");
    expect(edges.find((e) => e.id.startsWith("r:"))?.fromSide).toBeUndefined();
  });

  it("uses the radial dominant axis on a vertical parent edge", () => {
    const built = buildMapGraph(
      HAND,
      new Map([
        ["n0", { x: 0, y: 0 }],
        ["n0.1", { x: 0, y: -100 }],
      ]),
      { layout: "radial" },
    );
    expect(built.edges[0].fromSide).toBe("top");
    expect(built.edges[0].toSide).toBe("bottom");
  });
});
