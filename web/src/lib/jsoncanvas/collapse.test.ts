import { describe, expect, it } from "vitest";
import { nodeInside, collapsedContainedIds, boardGroups } from "./collapse";
import type { CanvasNode } from "./document";

const n = (o: Partial<CanvasNode>): CanvasNode =>
  ({ id: "x", type: "text", x: 0, y: 0, width: 10, height: 10, ...o }) as CanvasNode;

describe("collapse", () => {
  it("nodeInside is geometric containment (not identity)", () => {
    const g = n({ id: "g", type: "group", x: 0, y: 0, width: 100, height: 100 });
    expect(nodeInside(g, n({ id: "a", x: 10, y: 10, width: 10, height: 10 }))).toBe(true);
    expect(nodeInside(g, n({ id: "b", x: 95, y: 95, width: 10, height: 10 }))).toBe(false);
    expect(nodeInside(g, g)).toBe(false);
  });

  it("collapsedContainedIds hides nodes inside a collapsed group only", () => {
    const nodes = [
      n({ id: "g1", type: "group", x: 0, y: 0, width: 100, height: 100, "x-collapsed": true }),
      n({ id: "a", x: 10, y: 10, width: 10, height: 10 }),
      n({ id: "g2", type: "group", x: 200, y: 0, width: 100, height: 100 }),
      n({ id: "b", x: 210, y: 10, width: 10, height: 10 }),
    ];
    const hidden = collapsedContainedIds(nodes, new Set(["g1"]));
    expect(hidden.has("a")).toBe(true);
    expect(hidden.has("b")).toBe(false);
  });

  it("boardGroups reads the authored x-collapsed default", () => {
    const nodes = [n({ id: "g", type: "group", label: "Alpha", "x-collapsed": true })];
    expect(boardGroups(nodes)).toEqual([{ id: "g", label: "Alpha", collapsed: true }]);
  });
});
