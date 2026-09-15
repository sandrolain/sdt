import { describe, expect, it } from "vitest";
import {
  adaptGraph,
  adjacency,
  clusterOf,
  clusterPalette,
  DEFAULT_NODE_COLOR,
  linkKey,
  type GraphData,
} from "./graphModel";

const DATA: GraphData = {
  nodes: [
    {
      id: "a",
      title: "Alpha",
      type: "concept",
      status: "active",
      tags: ["backend/auth"],
      path: "context/wiki/a.md",
    },
    {
      id: "b",
      title: "Beta",
      type: "module",
      status: "draft",
      tags: ["ui"],
      path: "context/wiki/b.md",
    },
    { id: "c", title: "Gamma", type: "concept", status: "active", path: "context/wiki/c.md" },
  ],
  edges: [
    { source: "a", target: "b", verb: "depends_on", kind: "relation" },
    { source: "a", target: "c", verb: "refers_to", kind: "link" },
  ],
};

describe("clusterOf", () => {
  it("resolves type, tag-root and status", () => {
    expect(clusterOf(DATA.nodes[0], "type")).toBe("concept");
    expect(clusterOf(DATA.nodes[0], "tag-root")).toBe("backend");
    expect(clusterOf(DATA.nodes[1], "tag-root")).toBe("ui");
    expect(clusterOf(DATA.nodes[0], "status")).toBe("active");
    expect(clusterOf(DATA.nodes[2], "status")).toBe("active");
  });

  it("falls back to unknown when a dimension is missing", () => {
    expect(clusterOf(DATA.nodes[2], "type")).toBe("concept");
    expect(clusterOf({ id: "x", title: "X", path: "p" }, "type")).toBe("unknown");
    expect(clusterOf({ id: "x", title: "X", path: "p" }, "tag-root")).toBe("untagged");
  });
});

describe("clusterPalette", () => {
  it("assigns stable colors by sorted cluster id", () => {
    const palette = clusterPalette(DATA.nodes, "type");
    expect(palette.get("concept")).toBeDefined();
    expect(palette.get("module")).toBeDefined();
    expect(palette.get("concept")).not.toBe(palette.get("module"));
    const again = clusterPalette(DATA.nodes, "type");
    expect(again.get("concept")).toBe(palette.get("concept"));
  });
});

describe("adaptGraph", () => {
  it("maps nodes with cluster/color/val and links with color", () => {
    const g = adaptGraph(DATA, { clusterKey: "type" });
    expect(g.nodes).toHaveLength(3);
    expect(g.links).toHaveLength(2);
    const a = g.nodes.find((n) => n.id === "a")!;
    expect(a.cluster).toBe("concept");
    expect(a.color).not.toBe(DEFAULT_NODE_COLOR);
    expect(a.val).toBe(3); // degree 2 + 1
    expect(g.links[0].color).toBeTruthy();
  });

  it("filters links by verb and kind", () => {
    const g = adaptGraph(DATA, {
      clusterKey: "type",
      visibleVerbs: new Set(["refers_to"]),
      visibleKinds: new Set(["link"]),
    });
    expect(g.links).toHaveLength(1);
    expect(g.links[0].verb).toBe("refers_to");
  });

  it("reports the full verb/kind inventory", () => {
    const g = adaptGraph(DATA, { clusterKey: "type" });
    expect(g.allVerbs).toEqual(["depends_on", "refers_to"]);
    expect(g.allKinds).toEqual(["link", "relation"]);
  });
});

describe("linkKey", () => {
  it("is stable across endpoint object vs string forms", () => {
    const key = linkKey({ source: "a", target: "b", verb: "depends_on" });
    expect(key).toBe("a--depends_on-->b");
    expect(linkKey({ source: { id: "a" }, target: { id: "b" }, verb: "depends_on" } as never)).toBe(
      key,
    );
  });
});

describe("adjacency", () => {
  it("builds an undirected neighbor map", () => {
    const g = adaptGraph(DATA, { clusterKey: "type" });
    const adj = adjacency(g.links);
    expect([...adj.get("a")!].sort()).toEqual(["b", "c"]);
    expect([...adj.get("b")!]).toEqual(["a"]);
  });
});
