import { describe, expect, it } from "vitest";
import {
  adaptSemanticGraph,
  semanticNeighboursFor,
  SEMANTIC_EDGE_COLOR,
  type SemanticGraph,
} from "./semanticMap";

const GRAPH: SemanticGraph = {
  nodes: [
    { path: "context/notes/a.md", kind: "notes", title: "A", summary: "a summary" },
    { path: "context/analysis/b.md", kind: "analysis", title: "B" },
    { path: "context/tasks/t.md", kind: "tasks", title: "T" },
  ],
  edges: [
    { from: "context/notes/a.md", to: "context/analysis/b.md", score: 0.9 },
    { from: "context/notes/a.md", to: "context/tasks/t.md", score: 0.8 },
    { from: "context/analysis/b.md", to: "context/gone.md", score: 0.7 },
  ],
};

describe("adaptSemanticGraph", () => {
  it("maps nodes to the engine contract, id = path, label = title, group = kind", () => {
    const { nodes } = adaptSemanticGraph(GRAPH);
    expect(nodes).toHaveLength(3);
    const a = nodes.find((n) => n.id === "context/notes/a.md");
    expect(a?.label).toBe("A");
    expect(a?.group).toBe("notes");
    expect(a?.description).toBe("a summary");
  });

  it("keeps only edges whose both endpoints are present, tagged as semantic", () => {
    const { links } = adaptSemanticGraph(GRAPH);
    // a→b and a→t have both endpoints; b→gone.md (no node) is dropped.
    expect(links).toHaveLength(2);
    for (const l of links) {
      expect(l.type).toBe("semantic");
      expect(l.color).toBe(SEMANTIC_EDGE_COLOR);
    }
    expect(links.some((l) => l.target === "context/gone.md")).toBe(false);
  });

  it("colours nodes by kind with a stable palette", () => {
    const { palette, nodes } = adaptSemanticGraph(GRAPH);
    expect(palette.get("analysis")).toBeTruthy();
    expect(palette.get("notes")).toBeTruthy();
    expect(nodes.find((n) => n.group === "notes")?.color).toBe(palette.get("notes"));
  });

  it("degrades to an empty graph for an empty payload", () => {
    const { nodes, links } = adaptSemanticGraph({ nodes: [], edges: [] });
    expect(nodes).toHaveLength(0);
    expect(links).toHaveLength(0);
  });
});

describe("semanticNeighboursFor", () => {
  it("returns a document's neighbours ranked by score, symmetrically", () => {
    const a = semanticNeighboursFor(GRAPH, "context/notes/a.md");
    expect(a.map((n) => n.path)).toEqual(["context/analysis/b.md", "context/tasks/t.md"]);
    expect(a[0].title).toBe("B");
    const b = semanticNeighboursFor(GRAPH, "context/analysis/b.md");
    expect(b.map((n) => n.path)).toEqual(["context/notes/a.md"]);
  });

  it("returns nothing for a document outside the scoped map", () => {
    expect(semanticNeighboursFor(GRAPH, "context/plans/p.md")).toEqual([]);
  });
});
