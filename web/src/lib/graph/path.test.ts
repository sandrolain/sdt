import { describe, expect, it } from "vitest";
import { findShortestPath } from "./path";

const nodes = [{ id: "a" }, { id: "b" }, { id: "c" }, { id: "d" }];
const links = [
  { source: "a", target: "b", type: "refers_to" },
  { source: "b", target: "c", type: "depends_on" },
  { source: "c", target: "d", type: "refers_to" },
];

describe("findShortestPath", () => {
  it("finds a multi-hop path in node/edge order", () => {
    const path = findShortestPath("a", "c", nodes, links);
    expect(path?.nodeIds).toEqual(["a", "b", "c"]);
    expect(path?.links.map((l) => l.type)).toEqual(["refers_to", "depends_on"]);
  });

  it("finds a direct link", () => {
    const path = findShortestPath("a", "b", nodes, links);
    expect(path?.nodeIds).toEqual(["a", "b"]);
    expect(path?.links).toHaveLength(1);
  });

  it("returns null when disconnected", () => {
    const path = findShortestPath("a", "d", [{ id: "a" }, { id: "d" }], []);
    expect(path).toBeNull();
  });

  it("returns null for an unknown endpoint", () => {
    expect(findShortestPath("a", "zzz", nodes, links)).toBeNull();
  });
});
