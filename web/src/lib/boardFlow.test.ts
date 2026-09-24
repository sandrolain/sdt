// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { toFlowEdges, toFlowNodes } from "./boardFlow";
import { cardText, normalizeBoard } from "./canvas";

const MODEL = normalizeBoard({
  nodes: [
    { id: "g", type: "group", x: -10, y: -10, width: 300, height: 200, label: "Group" },
    { id: "a", type: "text", x: 0, y: 0, width: 120, height: 60, text: "Alpha" },
    { id: "f", type: "file", x: 220, y: 0, width: 120, height: 60, file: "context/wiki/alpha.md" },
  ],
  edges: [{ id: "e0", fromNode: "a", toNode: "f", label: "refers_to" }],
});

describe("toFlowNodes", () => {
  it("maps geometry, type and read-only flags", () => {
    const nodes = toFlowNodes(MODEL);
    const a = nodes.find((n) => n.id === "a");
    expect(a).toMatchObject({
      id: "a",
      type: "text",
      position: { x: 0, y: 0 },
      draggable: false,
      connectable: false,
      style: { width: 120, height: 60 },
      initialWidth: 120,
      initialHeight: 60,
    });
  });

  it("keeps group nodes behind the cards", () => {
    const nodes = toFlowNodes(MODEL);
    const group = nodes.find((n) => n.id === "g");
    const card = nodes.find((n) => n.id === "a");
    expect(group?.zIndex).toBeLessThan(card?.zIndex as number);
  });

  it("carries the open callback through node data", () => {
    const onOpen = vi.fn();
    const nodes = toFlowNodes(MODEL, onOpen);
    const data = nodes[0].data as {
      onOpen?: (node: unknown) => void;
      node: { id: string };
    };
    data.onOpen?.(data.node);
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: "g" }));
  });
});

describe("toFlowEdges", () => {
  it("maps endpoints, label and arrow marker", () => {
    const edges = toFlowEdges(MODEL);
    expect(edges[0]).toMatchObject({
      id: "e0",
      source: "a",
      target: "f",
      label: "refers_to",
      type: "default",
    });
    expect(edges[0].markerEnd).toMatchObject({ type: "arrowclosed" });
  });
});

describe("cardText", () => {
  it("prefers label, then text, then file title, then id", () => {
    expect(cardText({ id: "x", type: "text", x: 0, y: 0, width: 1, height: 1, label: "L" })).toBe(
      "L",
    );
    expect(cardText({ id: "x", type: "text", x: 0, y: 0, width: 1, height: 1, text: "T" })).toBe(
      "T",
    );
    expect(
      cardText({ id: "x", type: "file", x: 0, y: 0, width: 1, height: 1, file: "context/a.md" }),
    ).toBe("A");
    expect(cardText({ id: "x", type: "text", x: 0, y: 0, width: 1, height: 1 })).toBe("x");
  });
});
