import { describe, expect, it } from "vitest";
import { semanticEdgeBudget, thinEdges } from "./edges";
import type { SemanticEdge } from "./graph";

const edge = (from: string, to: string, score: number): SemanticEdge => ({ from, to, score });

describe("semanticEdgeBudget", () => {
  it("is one edge per node, floored and capped", () => {
    expect(semanticEdgeBudget(0)).toBe(24);
    expect(semanticEdgeBudget(10)).toBe(24);
    expect(semanticEdgeBudget(120)).toBe(120);
    expect(semanticEdgeBudget(5000)).toBe(240);
  });
});

describe("thinEdges", () => {
  const edges = [
    edge("a", "b", 0.4),
    edge("a", "c", 0.9),
    edge("b", "c", 0.7),
    edge("c", "d", 0.2),
  ];

  it("keeps the strongest and drops the rest", () => {
    expect(thinEdges(edges, 2).map((e) => `${e.from}-${e.to}`)).toEqual(["a-c", "b-c"]);
  });

  it("returns the set unchanged when the budget covers it", () => {
    expect(thinEdges(edges, 4)).toBe(edges);
    expect(thinEdges(edges, 99)).toBe(edges);
  });

  it("breaks a score tie by the ends, so the picture is stable", () => {
    const tied = [edge("z", "y", 0.5), edge("a", "b", 0.5), edge("a", "a", 0.5)];
    const first = thinEdges(tied, 2).map((e) => `${e.from}-${e.to}`);
    expect(first).toEqual(["a-a", "a-b"]);
    for (let i = 0; i < 5; i += 1) {
      expect(thinEdges(tied, 2).map((e) => `${e.from}-${e.to}`)).toEqual(first);
    }
  });
});
