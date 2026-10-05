import { describe, expect, it } from "vitest";
import { graphNeighbours } from "./neighbours";
import type { AdaptedEngineGraph } from "./adapter";

const adapted: AdaptedEngineGraph = {
  nodes: [
    { id: "a", label: "Alpha", group: "concept", color: "#cba6f7" },
    { id: "b", label: "Beta", group: "module", color: "#89b4fa" },
    { id: "c", label: "Gamma", group: "module", color: "#89b4fa" },
  ],
  links: [
    { source: "a", target: "b", type: "depends_on", kind: "relation" },
    { source: "c", target: "a", type: "depends_on", kind: "relation" },
    { source: "a", target: "c", type: "contains", kind: "link" },
  ],
  palette: new Map(),
  allVerbs: ["contains", "depends_on"],
  allKinds: ["link", "relation"],
};

describe("graphNeighbours", () => {
  it("groups incident edges by verb with in/out direction", () => {
    const groups = graphNeighbours(adapted, "a");
    expect(groups.map((g) => g.verb)).toEqual(["contains", "depends_on"]);
    expect(groups[0].items).toEqual([{ id: "c", label: "Gamma", direction: "out" }]);
    expect(groups[1].items).toEqual([
      { id: "b", label: "Beta", direction: "out" },
      { id: "c", label: "Gamma", direction: "in" },
    ]);
  });

  it("returns nothing without a selection", () => {
    expect(graphNeighbours(adapted, null)).toEqual([]);
  });
});
