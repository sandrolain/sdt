import { describe, expect, it } from "vitest";
import { canvasLayers, nodeLayer, normalizeCanvas } from "./document";

describe("normalizeCanvas", () => {
  it("reads geometry defaults and keeps the raw type", () => {
    const doc = normalizeCanvas({ nodes: [{ id: "a" }, { id: "b", type: "nested-canvas" }] });
    expect(doc.nodes[0]).toMatchObject({ id: "a", type: "text", width: 220, height: 110 });
    expect(doc.nodes[1].type).toBe("nested-canvas");
  });

  it("unwraps a canvas-file response and reads x-layer / x-layers", () => {
    const doc = normalizeCanvas({
      path: "context/wiki/x.canvas",
      canvas: {
        nodes: [{ id: "a", type: "text", "x-layer": 3 }],
        edges: [],
        "x-layers": [{ id: 3, name: "Deep" }],
      },
    });
    expect(nodeLayer(doc.nodes[0])).toBe(3);
    expect(doc["x-layers"]).toEqual([{ id: 3, name: "Deep" }]);
  });

  it("preserves unknown node/edge/root fields", () => {
    const doc = normalizeCanvas({
      nodes: [{ id: "a", type: "text", "x-foo": 7 }],
      edges: [{ id: "e", fromNode: "a", toNode: "a", "x-bar": "keep" }],
      "x-root": true,
    });
    expect((doc.nodes[0] as Record<string, unknown>)["x-foo"]).toBe(7);
    expect((doc.edges[0] as Record<string, unknown>)["x-bar"]).toBe("keep");
  });

  it("defaults x-layer to 0 when absent", () => {
    const doc = normalizeCanvas({ nodes: [{ id: "a", type: "text" }], edges: [] });
    expect(nodeLayer(doc.nodes[0])).toBe(0);
  });

  it("handles empty/foreign input", () => {
    expect(normalizeCanvas(null)).toEqual({ nodes: [], edges: [] });
    expect(normalizeCanvas("nope")).toEqual({ nodes: [], edges: [] });
  });
});

describe("canvasLayers", () => {
  it("lists the distinct x-layer values with x-layers names", () => {
    const doc = normalizeCanvas({
      nodes: [
        { id: "a", type: "text", "x-layer": 1 },
        { id: "b", type: "text", "x-layer": 0 },
        { id: "c", type: "text", "x-layer": 1 },
      ],
      edges: [],
      "x-layers": [{ id: 1, name: "Deep" }],
    });
    expect(canvasLayers(doc)).toEqual([
      { id: 0, name: "Level 0" },
      { id: 1, name: "Deep" },
    ]);
  });
});
