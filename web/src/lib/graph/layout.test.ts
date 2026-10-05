import { describe, expect, it } from "vitest";
import {
  circularPositions,
  computeLinkParams,
  groupCenters,
  hierarchyPositions,
  radialRings,
  type LayoutLinkLike,
  type LayoutNodeLike,
} from "./layout";

const node = (id: string, index: number, group: string, deg = 1): LayoutNodeLike => ({
  id,
  index,
  group,
  deg,
});

describe("computeLinkParams", () => {
  it("lengthens and weakens cross-group links in the groups layout", () => {
    const nodes = [node("a", 0, "x"), node("b", 1, "y")];
    const links: LayoutLinkLike[] = [{ a: 0, b: 1 }];
    const same = computeLinkParams("groups", nodes, links, new Int32Array([0, 0]), 38);
    const cross = computeLinkParams("groups", nodes, links, new Int32Array([0, 1]), 38);
    expect(cross.lDist[0]).toBeCloseTo(38 * 2.2, 5);
    expect(cross.lStr[0]).toBeLessThan(same.lStr[0]);
  });

  it("weakens links and honours the weight in the radial layout", () => {
    const nodes = [node("a", 0, "x"), node("b", 1, "x")];
    const base = computeLinkParams("radial", nodes, [{ a: 0, b: 1 }], new Int32Array([0, 0]), 38);
    const weighted = computeLinkParams(
      "radial",
      nodes,
      [{ a: 0, b: 1, weight: 2 }],
      new Int32Array([0, 0]),
      38,
    );
    expect(weighted.lStr[0]).toBeGreaterThan(base.lStr[0]);
  });
});

describe("groupCenters", () => {
  it("keeps a single group at the origin", () => {
    expect([...groupCenters(1, 5, 0)]).toEqual([0, 0, 0]);
  });

  it("spreads groups off the origin and flattens them in 2D", () => {
    const c = groupCenters(2, 5, 0);
    expect(Math.hypot(c[0], c[1], c[2])).toBeGreaterThan(0);
    expect(Math.hypot(c[3], c[4], c[5])).toBeGreaterThan(0);
    const flat = groupCenters(2, 5, 1);
    expect(flat[2]).toBeCloseTo(0, 6);
    expect(flat[5]).toBeCloseTo(0, 6);
  });
});

describe("radialRings", () => {
  it("gives each BFS depth an increasing radius", () => {
    const adj = [
      [{ o: 1, li: 0 }],
      [
        { o: 0, li: 0 },
        { o: 2, li: 1 },
      ],
      [{ o: 1, li: 1 }],
    ];
    const rings = radialRings(adj, 3, -1, [0, 1, 2], 64);
    expect(rings).not.toBeNull();
    expect(rings![0]).toBe(0);
    expect(rings![1]).toBeGreaterThan(0);
    expect(rings![2]).toBeGreaterThan(rings![1]);
  });

  it("returns null for an empty graph", () => {
    expect(radialRings([], 0, -1, [], 64)).toBeNull();
  });
});

describe("circularPositions", () => {
  it("is deterministic and separates clusters in z", () => {
    const nodes = [node("a", 0, "x"), node("b", 1, "x"), node("c", 2, "y")];
    const a = circularPositions(nodes);
    const b = circularPositions(nodes);
    expect([...a]).toEqual([...b]);
    expect(a[2]).toBe(0);
    expect(a[8]).toBe(40);
  });
});

describe("hierarchyPositions", () => {
  it("ranks a contained node below its container", () => {
    const nodes = [node("parent", 0, "x"), node("child", 1, "x")];
    const pos = hierarchyPositions(nodes, [{ a: 0, b: 1, type: "contains" }]);
    expect(pos[1]).not.toBeCloseTo(pos[4], 1);
  });
});
