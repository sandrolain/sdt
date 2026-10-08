import { describe, expect, it } from "vitest";
import { decorateBoardColours } from "./boardColors";
import type { BoardModel } from "./canvas";

describe("decorateBoardColours", () => {
  it("assigns a stable accent per cluster and a tone per edge kind", () => {
    const board = {
      nodes: [
        { id: "a", type: "text", x: 0, y: 0, width: 10, height: 10, "x-cluster": "module" },
        { id: "b", type: "text", x: 0, y: 0, width: 10, height: 10, "x-cluster": "entity" },
        { id: "g", type: "group", x: 0, y: 0, width: 10, height: 10, label: "module" },
        { id: "u", type: "text", x: 0, y: 0, width: 10, height: 10 },
      ],
      edges: [{ id: "e", fromNode: "a", toNode: "b", "x-kind": "relation" }],
    } as unknown as BoardModel;
    const out = decorateBoardColours(board);
    const byId = Object.fromEntries(out.nodes.map((n) => [n.id, n]));
    expect(byId["a"].color).toBeTruthy();
    expect(byId["b"].color).toBeTruthy();
    expect(byId["a"].color).not.toBe(byId["b"].color);
    expect(byId["g"].color).toBeTruthy(); // a group box gets its own tone
    expect(byId["u"].color).toBeUndefined(); // unclustered keeps the default
    expect(out.edges[0].color).toBeTruthy();
  });

  it("degrades an unknown edge kind to no colour", () => {
    const board = {
      nodes: [],
      edges: [{ id: "e", fromNode: "a", toNode: "b", "x-kind": "weird" }],
    } as unknown as BoardModel;
    expect(decorateBoardColours(board).edges[0].color).toBeUndefined();
  });
});
