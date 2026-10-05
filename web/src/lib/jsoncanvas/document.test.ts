import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import {
  canvasLayers,
  findNodeRecursively,
  nestedCanvas,
  nodeLayer,
  normalizeCanvas,
} from "./document";

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

describe("nested canvas", () => {
  const NESTED = {
    nodes: [
      { id: "root", type: "text", "x-foo": 1 },
      {
        id: "nc",
        type: "nested-canvas",
        title: "Investigation",
        canvas: {
          nodes: [{ id: "inner", type: "text", "x-bar": 2 }],
          edges: [{ id: "ie", fromNode: "inner", toNode: "inner" }],
        },
      },
      { id: "empty", type: "nested-canvas", title: "Empty" },
    ],
    edges: [],
  };

  it("normalizes the embedded child recursively and keeps unknown fields", () => {
    const doc = normalizeCanvas(NESTED);
    const child = nestedCanvas(doc.nodes[1]);
    expect(child).not.toBeNull();
    expect(child!.nodes[0].id).toBe("inner");
    expect(child!.nodes[0].type).toBe("text");
    expect(child!.nodes[0]["x-bar"]).toBe(2);
    expect(child!.edges[0].id).toBe("ie");
    expect((doc.nodes[1] as Record<string, unknown>)["x-foo"]).toBeUndefined();
    expect(doc.nodes[1].title).toBe("Investigation");
  });

  it("gives a node without a canvas an empty child", () => {
    const doc = normalizeCanvas(NESTED);
    expect(nestedCanvas(doc.nodes[2])).toEqual({ nodes: [], edges: [] });
  });

  it("finds a deep node id and returns the nested node ids to enter", () => {
    const doc = normalizeCanvas(NESTED);
    expect(findNodeRecursively(doc, "root")).toEqual([]);
    expect(findNodeRecursively(doc, "inner")).toEqual(["nc"]);
    expect(findNodeRecursively(doc, "missing")).toBeNull();
  });

  it("normalizes the committed fixture and resolves a two-level-deep id", () => {
    const raw = readFileSync(new URL("./fixtures/nested.canvas", import.meta.url), "utf8");
    const doc = normalizeCanvas(JSON.parse(raw));
    expect((doc.nodes.find((n) => n.id === "t") as Record<string, unknown>)["x-foo"]).toBe("keep");
    expect(doc.nodes.find((n) => n.id === "svg1")?.type).toBe("svg");
    expect(findNodeRecursively(doc, "c2")).toEqual(["nc1", "nc2"]);
  });

  it("normalizes a deeply nested document without unbounded cost", () => {
    let doc: unknown = { nodes: [{ id: "leaf", type: "text" }], edges: [] };
    for (let i = 0; i < 50; i++) {
      doc = {
        nodes: [{ id: `n${i}`, type: "nested-canvas", title: `L${i}`, canvas: doc }],
        edges: [],
      };
    }
    const norm = normalizeCanvas(doc);
    expect(findNodeRecursively(norm, "leaf")).toHaveLength(50);
  });
});
