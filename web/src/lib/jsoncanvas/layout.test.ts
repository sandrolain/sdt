import { describe, expect, it } from "vitest";
import type { CanvasDocument, CanvasNode } from "./document";
import { layoutCanvas } from "./layout";

function node(id: string, x: number, y: number, w = 200, h = 100): CanvasNode {
  return { id, type: "text", x, y, width: w, height: h };
}

function group(id: string, x: number, y: number, w: number, h: number): CanvasNode {
  return { id, type: "group", x, y, width: w, height: h, label: id };
}

describe("layoutCanvas", () => {
  it("packs ungrouped nodes into a deterministic grid", () => {
    const doc: CanvasDocument = {
      nodes: [node("a", 0, 0), node("b", 500, 0), node("c", 0, 500), node("d", 500, 500)],
      edges: [],
    };
    const pos = layoutCanvas(doc);
    expect(pos.size).toBe(4);
    expect(pos.get("a")).toEqual({ x: 0, y: 0 });
    // two columns for four nodes
    expect(pos.get("c")).toEqual({ x: 0, y: 140 });
  });

  it("arranges a group's children inside it and resizes the group", () => {
    const doc: CanvasDocument = {
      nodes: [group("g", 0, 0, 400, 300), node("a", 10, 10, 100, 50), node("b", 20, 20, 100, 50)],
      edges: [],
    };
    const pos = layoutCanvas(doc);
    // the group grows to fit its two children
    expect(pos.get("g")!.width).toBeGreaterThan(100);
    // children start at the group's padding, inside it
    expect(pos.get("a")!.x).toBe(48);
    expect(pos.get("a")!.y).toBe(48);
  });

  it("is stable across runs", () => {
    const doc: CanvasDocument = {
      nodes: [node("a", 0, 0), node("b", 300, 0), node("c", 0, 300)],
      edges: [],
    };
    expect([...layoutCanvas(doc)]).toEqual([...layoutCanvas(doc)]);
  });
});
