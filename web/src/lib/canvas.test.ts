import { describe, expect, it } from "vitest";
import { normalizeBoard } from "./canvas";

const API_BOARD = {
  nodes: [
    { id: "a", type: "text", x: 0, y: 0, width: 100, height: 50, text: "A" },
    { id: "b", type: "text", x: 200, y: 100, width: 100, height: 50, text: "B" },
  ],
  edges: [
    { id: "e0", fromNode: "a", toNode: "b", fromSide: "right", toSide: "left", label: "refers_to" },
  ],
};

describe("normalizeBoard", () => {
  it("normalizes an API board response", () => {
    const model = normalizeBoard(API_BOARD);
    expect(model.nodes).toHaveLength(2);
    expect(model.edges[0].label).toBe("refers_to");
  });

  it("unwraps a canvas-file response", () => {
    const model = normalizeBoard({ path: "context/wiki/x.canvas", canvas: API_BOARD });
    expect(model.nodes).toHaveLength(2);
  });

  it("applies geometry defaults and preserves the raw node type", () => {
    const model = normalizeBoard({ nodes: [{ id: "a" }, { type: "nope" }, null], edges: [{}] });
    expect(model.nodes[0]).toMatchObject({
      id: "a",
      type: "text",
      x: 0,
      y: 0,
      width: 220,
      height: 110,
    });
    expect(model.nodes[1].id).toBe("1");
    expect(model.nodes[1].type).toBe("nope");
    expect(model.edges[0].fromNode).toBe("");
  });

  it("carries x-layer and preserves unknown fields", () => {
    const model = normalizeBoard({ nodes: [{ id: "a", type: "text", "x-layer": 2, "x-foo": 1 }] });
    expect(model.nodes[0]["x-layer"]).toBe(2);
    expect((model.nodes[0] as Record<string, unknown>)["x-foo"]).toBe(1);
  });

  it("handles empty/foreign input", () => {
    expect(normalizeBoard(null)).toEqual({ nodes: [], edges: [] });
    expect(normalizeBoard("nope")).toEqual({ nodes: [], edges: [] });
  });
});
