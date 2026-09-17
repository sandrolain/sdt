import { describe, expect, it } from "vitest";
import { graphToolsReducer, initialGraphTools, visibleSet } from "./graphTools";

describe("graphToolsReducer", () => {
  it("sets mode, layout and cluster key", () => {
    let s = graphToolsReducer(initialGraphTools, { type: "mode", value: "3d" });
    s = graphToolsReducer(s, { type: "layout", value: "hierarchy" });
    s = graphToolsReducer(s, { type: "clusterKey", value: "tag-root" });
    expect(s.mode).toBe("3d");
    expect(s.layout).toBe("hierarchy");
    expect(s.clusterKey).toBe("tag-root");
  });

  it("toggles verbs and kinds", () => {
    let s = graphToolsReducer(initialGraphTools, { type: "toggleVerb", value: "refers_to" });
    s = graphToolsReducer(s, { type: "toggleKind", value: "link" });
    expect(s.hiddenVerbs).toEqual(["refers_to"]);
    expect(s.hiddenKinds).toEqual(["link"]);
    s = graphToolsReducer(s, { type: "toggleVerb", value: "refers_to" });
    expect(s.hiddenVerbs).toEqual([]);
  });

  it("sets the hidden sets directly (multi-select)", () => {
    let s = graphToolsReducer(initialGraphTools, {
      type: "setHiddenVerbs",
      value: ["refers_to"],
    });
    s = graphToolsReducer(s, { type: "setHiddenKinds", value: ["link", "relation"] });
    expect(s.hiddenVerbs).toEqual(["refers_to"]);
    expect(s.hiddenKinds).toEqual(["link", "relation"]);
  });

  it("toggles labels, focuses and resets", () => {
    let s = graphToolsReducer(initialGraphTools, { type: "labels", value: false });
    expect(s.showLabels).toBe(false);
    s = graphToolsReducer(s, { type: "focus", value: "a" });
    expect(s.focusId).toBe("a");
    s = graphToolsReducer(s, { type: "focus", value: null });
    expect(s.focusId).toBeNull();
    s = graphToolsReducer(s, { type: "reset" });
    expect(s).toEqual(initialGraphTools);
  });
});

describe("visibleSet", () => {
  it("returns undefined when nothing is hidden", () => {
    expect(visibleSet(["a", "b"], [])).toBeUndefined();
  });

  it("filters hidden entries", () => {
    expect([...visibleSet(["a", "b", "c"], ["b"])!]).toEqual(["a", "c"]);
  });
});
