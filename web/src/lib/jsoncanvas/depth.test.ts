import { describe, expect, it } from "vitest";
import { nodeDepth, planeLayer } from "./depth";
import type { CanvasNode } from "./document";

const n = (o: Partial<CanvasNode>): CanvasNode =>
  ({ id: "x", type: "text", x: 0, y: 0, width: 10, height: 10, ...o }) as CanvasNode;

describe("depth", () => {
  it("a finite x-z supersedes the x-layer ladder", () => {
    expect(nodeDepth(n({ "x-layer": 2, "x-z": 250 }), 100)).toBe(250);
    expect(nodeDepth(n({ "x-layer": 2 }), 100)).toBe(200);
    expect(nodeDepth(n({ "x-layer": 3, "x-z": Number.NaN }), 100)).toBe(300);
  });

  it("planeLayer prefers x-layer, else the nearest lower plane from x-z", () => {
    expect(planeLayer(n({ "x-layer": 2, "x-z": 250 }), 100)).toBe(2);
    expect(planeLayer(n({ "x-z": 250 }), 100)).toBe(2);
    expect(planeLayer(n({ "x-layer": 4 }), 100)).toBe(4);
  });
});
