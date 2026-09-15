import { describe, expect, it } from "vitest";
import { adaptGraph, type GraphData } from "./graphModel";
import {
  computeHighlight,
  linkVisual,
  nodeVisual,
  selectionReducer,
  initialSelection,
} from "./graphSelection";

const DATA: GraphData = {
  nodes: [
    { id: "a", title: "A", type: "concept", path: "context/wiki/a.md" },
    { id: "b", title: "B", type: "module", path: "context/wiki/b.md" },
    { id: "c", title: "C", type: "module", path: "context/wiki/c.md" },
    { id: "d", title: "D", type: "pattern", path: "context/wiki/d.md" },
  ],
  edges: [
    { source: "a", target: "b", verb: "depends_on", kind: "relation" },
    { source: "b", target: "c", verb: "refers_to", kind: "link" },
    { source: "c", target: "d", verb: "refers_to", kind: "link" },
  ],
};

const g = adaptGraph(DATA, { clusterKey: "type" });

describe("selectionReducer", () => {
  it("selects, hovers and clears", () => {
    let s = selectionReducer(initialSelection, { type: "select", id: "a" });
    s = selectionReducer(s, { type: "hover", id: "b" });
    expect(s).toEqual({ selected: "a", hovered: "b" });
    expect(selectionReducer(s, { type: "clear" })).toEqual(initialSelection);
  });
});

describe("computeHighlight", () => {
  it("is inactive without a focus", () => {
    const h = computeHighlight(g.links, initialSelection);
    expect(h.active).toBe(false);
    expect(nodeVisual("a", h)).toEqual({ alpha: 1, emphasis: false });
  });

  it("ranks selected, neighbors, blast radius and the rest", () => {
    const h = computeHighlight(g.links, { selected: "b", hovered: null }, 2);
    expect(h.active).toBe(true);
    expect(h.selected.has("b")).toBe(true);
    expect(h.neighbors.has("a")).toBe(true);
    expect(h.neighbors.has("c")).toBe(true);
    expect(h.blast.has("d")).toBe(true); // 2 hops from b
    expect(nodeVisual("b", h).emphasis).toBe(true);
    expect(nodeVisual("a", h).alpha).toBe(0.9);
    expect(nodeVisual("d", h).alpha).toBe(0.55);
  });

  it("highlights edges touching the focus", () => {
    const h = computeHighlight(g.links, { selected: "a", hovered: null });
    expect(h.edges.size).toBe(1);
    expect(linkVisual({ source: "a", target: "b", verb: "depends_on" }, h).alpha).toBe(1);
    expect(linkVisual({ source: "c", target: "d", verb: "refers_to" }, h).alpha).toBeLessThan(0.5);
  });
});
