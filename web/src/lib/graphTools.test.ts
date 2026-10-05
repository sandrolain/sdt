import { describe, expect, it } from "vitest";
import { graphToolsReducer, initialGraphTools } from "./graphTools";

describe("graphToolsReducer", () => {
  it("sets mode, layout and cluster key", () => {
    let s = graphToolsReducer(initialGraphTools, { type: "mode", value: "3d" });
    s = graphToolsReducer(s, { type: "layout", value: "circular" });
    s = graphToolsReducer(s, { type: "clusterKey", value: "status" });
    expect(s.mode).toBe("3d");
    expect(s.layout).toBe("circular");
    expect(s.clusterKey).toBe("status");
  });

  it("toggles each filter dimension independently", () => {
    let s = graphToolsReducer(initialGraphTools, { type: "toggleGroup", value: "concept" });
    s = graphToolsReducer(s, { type: "toggleRelation", value: "part_of" });
    s = graphToolsReducer(s, { type: "toggleKind", value: "link" });
    expect(s.hiddenGroups).toEqual(["concept"]);
    expect(s.hiddenRelations).toEqual(["part_of"]);
    expect(s.hiddenKinds).toEqual(["link"]);
    s = graphToolsReducer(s, { type: "toggleGroup", value: "concept" });
    expect(s.hiddenGroups).toEqual([]);
  });

  it("clears the filters without touching view state", () => {
    let s = graphToolsReducer(initialGraphTools, { type: "mode", value: "3d" });
    s = graphToolsReducer(s, { type: "toggleGroup", value: "concept" });
    s = graphToolsReducer(s, { type: "toggleRelation", value: "part_of" });
    s = graphToolsReducer(s, { type: "toggleKind", value: "link" });
    s = graphToolsReducer(s, { type: "clearFilters" });
    expect(s.hiddenGroups).toEqual([]);
    expect(s.hiddenRelations).toEqual([]);
    expect(s.hiddenKinds).toEqual([]);
    expect(s.mode).toBe("3d");
  });

  it("toggles labels, centrality and neighbors, and resets", () => {
    let s = graphToolsReducer(initialGraphTools, { type: "labels", value: false });
    s = graphToolsReducer(s, { type: "centrality", value: true });
    s = graphToolsReducer(s, { type: "neighbors", value: true });
    expect(s.showLabels).toBe(false);
    expect(s.centrality).toBe(true);
    expect(s.neighborsOnly).toBe(true);
    s = graphToolsReducer(s, { type: "reset" });
    expect(s).toEqual(initialGraphTools);
  });
});
