import { describe, expect, it } from "vitest";
import { adaptGraph, type GraphData } from "./graphModel";
import { applyLayout, mulberry32 } from "./graphLayout";

const DATA: GraphData = {
  nodes: [
    { id: "root", title: "Root", type: "concept", path: "context/wiki/root.md" },
    { id: "child", title: "Child", type: "module", path: "context/wiki/child.md" },
    { id: "other", title: "Other", type: "concept", path: "context/wiki/other.md" },
  ],
  edges: [{ source: "child", target: "root", verb: "part_of", kind: "relation" }],
};

function dataWithClusters() {
  return adaptGraph(DATA, { clusterKey: "type" });
}

describe("mulberry32", () => {
  it("is deterministic for a given seed", () => {
    const a = mulberry32(42);
    const b = mulberry32(42);
    expect([a(), a(), a()]).toEqual([b(), b(), b()]);
    expect(mulberry32(1)()).not.toBe(mulberry32(2)());
  });
});

describe("applyLayout", () => {
  it("seeds force positions deterministically and preserves identity", () => {
    const g = dataWithClusters();
    const first = applyLayout(g.nodes, g.links, "force", { seed: 7 });
    const second = applyLayout(g.nodes, g.links, "force", { seed: 7 });
    expect(first.map((n) => [n.id, n.x, n.y])).toEqual(second.map((n) => [n.id, n.x, n.y]));
    expect(first.map((n) => n.id).sort()).toEqual(["child", "other", "root"]);
    for (const n of first) expect(Number.isFinite(n.x)).toBe(true);
  });

  it("changes positions with a different seed", () => {
    const g = dataWithClusters();
    const ax = applyLayout(g.nodes, g.links, "force", { seed: 1 }).map((n) => n.x);
    const bx = applyLayout(g.nodes, g.links, "force", { seed: 2 }).map((n) => n.x);
    expect(ax).not.toEqual(bx);
  });

  it("places circular nodes on a ring", () => {
    const g = dataWithClusters();
    const laid = applyLayout(g.nodes, g.links, "circular");
    for (const n of laid) {
      const r = Math.hypot(n.x ?? 0, n.y ?? 0);
      expect(r).toBeCloseTo(220, 3);
    }
  });

  it("orders hierarchy children below their parent", () => {
    const g = dataWithClusters();
    const laid = applyLayout(g.nodes, g.links, "hierarchy");
    const root = laid.find((n) => n.id === "root")!;
    const child = laid.find((n) => n.id === "child")!;
    expect(child.y).toBeGreaterThan(root.y ?? 0);
  });

  it.each(["radial", "circular", "hierarchy"] as const)(
    "unbinds drag pins in %s so a dragged node rejoins the arrangement",
    (kind) => {
      const g = dataWithClusters();
      const pinned = g.nodes.find((n) => n.id === "root")!;
      pinned.fx = 999;
      pinned.fy = 999;
      pinned.fz = 999;
      const laid = applyLayout(g.nodes, g.links, kind);
      const root = laid.find((n) => n.id === "root")!;
      expect(root.fx).toBeUndefined();
      expect(root.fy).toBeUndefined();
      expect(root.fz).toBeUndefined();
      expect([root.x, root.y]).not.toEqual([999, 999]);
    },
  );

  it("positions nodes in place, preserving object identity per call", () => {
    const g = dataWithClusters();
    const input = [...g.nodes];
    const laid = applyLayout(g.nodes, g.links, "force", { seed: 3 });
    expect(laid).toBe(g.nodes);
    for (const original of input) {
      const positioned = laid.find((n) => n.id === original.id)!;
      expect(positioned).toBe(original);
      expect(original.fx).toBeUndefined();
    }
  });
});
