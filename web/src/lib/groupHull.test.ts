import { polygonContains } from "d3-polygon";
import { describe, expect, it } from "vitest";
import { type NodeRectLike } from "./boundaries";
import { GROUP_PADDING, groupColor, groupHulls } from "./groupHull";
import type { Group } from "./groups";

/** A group of id-less members: the hull only reads the rect map by node id. */
function group(id: string, ids: string[]): Group {
  return {
    id,
    nodes: ids.map((nodeId) => ({ id: nodeId }) as unknown as Group["nodes"][number]),
  };
}

function rects(entries: Record<string, NodeRectLike>): Map<string, NodeRectLike> {
  return new Map(Object.entries(entries));
}

type Point = [number, number];

function pointsOf(path: string): Point[] {
  const nums = path.match(/-?\d+(?:\.\d+)?/g)?.map(Number) ?? [];
  const points: Point[] = [];
  for (let i = 0; i < nums.length; i += 2) points.push([nums[i], nums[i + 1]]);
  return points;
}

function cornersOf(rect: NodeRectLike): Point[] {
  return [
    [rect.x, rect.y],
    [rect.x + rect.width, rect.y],
    [rect.x + rect.width, rect.y + rect.height],
    [rect.x, rect.y + rect.height],
  ];
}

describe("groupColor", () => {
  it("is deterministic and order-independent", () => {
    expect(groupColor("auth")).toBe(groupColor("auth"));
    expect(groupColor("auth")).not.toBe(groupColor("core"));
  });
});

describe("groupHulls", () => {
  it("skips groups whose members carry no rect", () => {
    expect(groupHulls([group("empty", ["a"])], rects({}))).toEqual([]);
  });

  it("falls back to a padded rect for a single member", () => {
    const rect = { x: 10, y: 10, width: 50, height: 20 };
    const [hull] = groupHulls([group("one", ["a"])], rects({ a: rect }));
    const points = pointsOf(hull.path);
    expect(points).toHaveLength(4);
    for (const [x, y] of cornersOf(rect)) {
      expect(polygonContains(points, [x, y])).toBe(true);
    }
    expect(Math.min(...points.map((p) => p[0]))).toBe(10 - GROUP_PADDING);
    expect(Math.min(...points.map((p) => p[1]))).toBe(10 - GROUP_PADDING);
    expect(hull.labelX).toBe(10 - GROUP_PADDING + 8);
    expect(hull.labelY).toBe(10 - GROUP_PADDING + 15);
  });

  it("wraps scattered members in one hull that contains every corner", () => {
    const a = { x: 0, y: 0, width: 40, height: 20 };
    const b = { x: 200, y: 120, width: 40, height: 20 };
    const c = { x: 80, y: 300, width: 40, height: 20 };
    const [hull] = groupHulls([group("scattered", ["a", "b", "c"])], rects({ a, b, c }));
    const points = pointsOf(hull.path);
    expect(points.length).toBeGreaterThanOrEqual(3);
    expect(hull.path.endsWith("Z")).toBe(true);
    for (const rect of [a, b, c]) {
      for (const corner of cornersOf(rect)) {
        expect(polygonContains(points, corner)).toBe(true);
      }
    }
  });

  it("keeps one hull per group and preserves first-seen order", () => {
    const hulls = groupHulls(
      [group("one", ["a"]), group("two", ["b"])],
      rects({
        a: { x: 0, y: 0, width: 10, height: 10 },
        b: { x: 50, y: 0, width: 10, height: 10 },
      }),
    );
    expect(hulls.map((h) => h.id)).toEqual(["one", "two"]);
    expect(hulls[0].color).toBe(groupColor("one"));
  });

  it("reads the member rects by node id, so a pruned member is skipped", () => {
    const [hull] = groupHulls(
      [group("g", ["a", "hidden"])],
      rects({ a: { x: 0, y: 0, width: 10, height: 10 } }),
    );
    expect(pointsOf(hull.path)).toHaveLength(4);
  });
});
