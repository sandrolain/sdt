import { describe, expect, it } from "vitest";
import { boardBounds, cardRoute, nodeCenter, normalizeBoard } from "./canvas";

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

  it("applies geometry defaults and skips malformed entries", () => {
    const model = normalizeBoard({ nodes: [{ id: "a" }, { type: "nope" }, null], edges: [{}] });
    expect(model.nodes[0]).toEqual({
      id: "a",
      type: "text",
      x: 0,
      y: 0,
      width: 220,
      height: 110,
      color: undefined,
      text: undefined,
      file: undefined,
      url: undefined,
      label: undefined,
    });
    expect(model.nodes[1].id).toBe("1");
    expect(model.edges[0].fromNode).toBe("");
  });

  it("handles empty/foreign input", () => {
    expect(normalizeBoard(null)).toEqual({ nodes: [], edges: [] });
    expect(normalizeBoard("nope")).toEqual({ nodes: [], edges: [] });
  });
});

describe("boardBounds / nodeCenter", () => {
  it("computes the union box", () => {
    const b = boardBounds(normalizeBoard(API_BOARD));
    expect(b).toMatchObject({ minX: 0, minY: 0, maxX: 300, maxY: 150, width: 300, height: 150 });
  });

  it("returns a zero box when empty", () => {
    expect(boardBounds({ nodes: [], edges: [] }).width).toBe(0);
  });

  it("centers a node", () => {
    expect(nodeCenter({ id: "a", type: "text", x: 10, y: 20, width: 100, height: 50 })).toEqual({
      x: 60,
      y: 45,
    });
  });
});

describe("cardRoute", () => {
  it("routes file cards to docs and text cards to wiki", () => {
    expect(cardRoute({ id: "topic", type: "text", x: 0, y: 0, width: 1, height: 1 })).toBe(
      "/wiki/topic",
    );
    expect(
      cardRoute({ id: "n", type: "file", x: 0, y: 0, width: 1, height: 1, file: "wiki/x.md" }),
    ).toBe("/docs/context/wiki/x.md");
    expect(cardRoute({ id: "g", type: "group", x: 0, y: 0, width: 1, height: 1 })).toBeNull();
  });
});
