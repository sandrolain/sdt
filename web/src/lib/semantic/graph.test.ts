import { describe, expect, it } from "vitest";
import {
  adaptSemanticGraph,
  semanticNeighboursFor,
  SEMANTIC_EDGE_COLOR,
  type SemanticGraph,
} from "./graph";

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

// The same graph with the mode-selector overlay (projection + clusters).
const OVERLAY: SemanticGraph = {
  ...GRAPH,
  projection: {
    "context/notes/a.md": [1, 0],
    "context/analysis/b.md": [0, 1],
    "context/tasks/t.md": [0.5, 0.5],
  },
  clusters: {
    "context/notes/a.md": 0,
    "context/analysis/b.md": 1,
    "context/tasks/t.md": 0,
  },
};

describe("adaptSemanticGraph modes", () => {
  const node = (g: ReturnType<typeof adaptSemanticGraph>, id: string) =>
    g.nodes.find((n) => n.id === id)!;

  it("defaults to the edge map: kind groups, edges kept, no coordinates", () => {
    const g = adaptSemanticGraph(OVERLAY);
    expect(g.links).toHaveLength(2);
    expect(node(g, "context/notes/a.md").group).toBe("notes");
    expect(node(g, "context/notes/a.md").x).toBeUndefined();
  });

  it("Projection positions nodes by the projection and drops the edges", () => {
    const g = adaptSemanticGraph(OVERLAY, "projection");
    expect(g.links).toHaveLength(0);
    for (const n of g.nodes) {
      expect(typeof n.x).toBe("number");
      expect(typeof n.y).toBe("number");
    }
    // relative geometry preserved: a sits right of b on the first component
    expect(node(g, "context/notes/a.md").x as number).toBeGreaterThan(
      node(g, "context/analysis/b.md").x as number,
    );
  });

  it("Clusters groups and colours by cluster, keeping the edges", () => {
    const g = adaptSemanticGraph(OVERLAY, "clusters");
    expect(g.links).toHaveLength(2);
    const a = node(g, "context/notes/a.md");
    const t = node(g, "context/tasks/t.md");
    const b = node(g, "context/analysis/b.md");
    expect(a.group).toBe("cluster 0");
    expect(t.group).toBe("cluster 0");
    expect(a.color).toBe(t.color);
    expect(b.group).toBe("cluster 1");
    expect(b.color).not.toBe(a.color);
  });

  it("names a cluster from the payload's labels, and falls back to the index", () => {
    const named = adaptSemanticGraph(
      { ...OVERLAY, clusterLabels: { "0": "Notes · Beta", "1": "Decisions 1" } },
      "clusters",
    );
    expect(node(named, "context/notes/a.md").group).toBe("Notes · Beta");
    expect(node(named, "context/analysis/b.md").group).toBe("Decisions 1");
    // The same index keeps one colour whichever name it carries.
    expect(node(named, "context/notes/a.md").color).toBe(node(named, "context/tasks/t.md").color);

    // No labels at all: the index is the honest fallback, never a blank group.
    const bare = adaptSemanticGraph(OVERLAY, "clusters");
    expect(node(bare, "context/notes/a.md").group).toBe("cluster 0");

    // A cluster the payload did not name keeps its index rather than borrowing one.
    const partial = adaptSemanticGraph(
      { ...OVERLAY, clusterLabels: { "0": "Notes · Beta" } },
      "clusters",
    );
    expect(node(partial, "context/analysis/b.md").group).toBe("cluster 1");
  });

  it("Combined positions by projection and colours by cluster", () => {
    const g = adaptSemanticGraph(OVERLAY, "combined");
    expect(g.links).toHaveLength(2);
    const a = node(g, "context/notes/a.md");
    expect(typeof a.x).toBe("number");
    expect(a.group).toBe("cluster 0");
  });
});
