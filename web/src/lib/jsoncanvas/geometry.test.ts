import { describe, expect, it } from "vitest";
import { anchor, arrow, autoSides, edgeGeom, inside, SIDES } from "./geometry";
import type { CanvasNode } from "./document";

const node = (over: Partial<CanvasNode> = {}): CanvasNode => ({
  id: "n",
  type: "text",
  x: 0,
  y: 0,
  width: 100,
  height: 100,
  ...over,
});

describe("geometry", () => {
  it("anchors on each side", () => {
    const n = node({ x: 10, y: 20, width: 100, height: 50 });
    expect(anchor(n, "left")).toEqual({ x: 10, y: 45 });
    expect(anchor(n, "right")).toEqual({ x: 110, y: 45 });
    expect(anchor(n, "top")).toEqual({ x: 60, y: 20 });
    expect(anchor(n, "bottom")).toEqual({ x: 60, y: 70 });
    expect(SIDES.right).toEqual([1, 0]);
  });

  it("autoSides picks the facing horizontal sides", () => {
    expect(autoSides(node(), node({ x: 300 }))).toEqual(["right", "left"]);
    expect(autoSides(node({ x: 300 }), node())).toEqual(["left", "right"]);
    expect(autoSides(node(), node({ y: 300, width: 100, height: 20 }))).toEqual(["bottom", "top"]);
  });

  it("inside reports group containment", () => {
    const g = node({ id: "g", type: "group", x: 0, y: 0, width: 200, height: 200 });
    expect(inside(g, node({ id: "a", x: 10, y: 10 }))).toBe(true);
    expect(inside(g, node({ id: "a", x: 150, y: 150 }))).toBe(false);
    expect(inside(g, g)).toBe(false);
  });

  it("edgeGeom honours explicit sides and the mid anchor", () => {
    const a = node({ id: "a" });
    const b = node({ id: "b", x: 300 });
    const byId = { a, b };
    const g = edgeGeom(
      { id: "e", fromNode: "a", fromSide: "right", toNode: "b", toSide: "left" },
      byId,
    );
    expect(g).not.toBeNull();
    expect(g?.p).toEqual({ x: 100, y: 50 });
    expect(g?.q).toEqual({ x: 300, y: 50 });
    expect(g?.mid).toEqual({ x: 200, y: 50 });
    expect(g?.d).toBe("M100,50 C180,50 220,50 300,50");
  });

  it("edgeGeom falls back to autoSides and returns null on a missing endpoint", () => {
    const a = node({ id: "a" });
    const b = node({ id: "b", x: 300 });
    const auto = edgeGeom({ id: "e", fromNode: "a", toNode: "b" }, { a, b });
    expect(auto?.p).toEqual({ x: 100, y: 50 });
    expect(edgeGeom({ id: "e", fromNode: "a", toNode: "z" }, { a, b })).toBeNull();
  });

  it("builds an arrow polygon from the normal", () => {
    expect(arrow({ x: 0, y: 0 }, [1, 0], 12)).toBe("0,0 12,6 12,-6");
  });
});
