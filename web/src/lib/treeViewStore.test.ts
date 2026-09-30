// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { resetTreeFilter, setGroupMode, setHiddenStates } from "./treeFilterStore";
import { resetTreeSort, setTreeSortKey } from "./treeSortStore";
import {
  clearTreeView,
  loadTreeView,
  saveTreeView,
  TREE_VIEW_KEY,
  TREE_VIEW_VERSION,
} from "./treeViewStore";

describe("treeViewStore", () => {
  beforeEach(() => {
    localStorage.clear();
    clearTreeView();
  });

  it("round-trips the slices it is given", () => {
    saveTreeView({ sortKey: "title_asc", groupMode: "full" });
    expect(loadTreeView()).toEqual({ sortKey: "title_asc", groupMode: "full" });
  });

  it("merges a slice without touching the others", () => {
    saveTreeView({ sortKey: "title_asc" });
    saveTreeView({ groupMode: "flat" });
    expect(loadTreeView()).toEqual({ sortKey: "title_asc", groupMode: "flat" });
  });

  it("clears a slice on undefined and drops a record left empty", () => {
    saveTreeView({ sortKey: "title_asc", groupMode: "flat" });
    saveTreeView({ sortKey: undefined });
    expect(loadTreeView()).toEqual({ groupMode: "flat" });
    saveTreeView({ groupMode: undefined });
    expect(localStorage.getItem(TREE_VIEW_KEY)).toBeNull();
  });

  it("drops a record written by another version", () => {
    localStorage.setItem(
      TREE_VIEW_KEY,
      JSON.stringify({ version: TREE_VIEW_VERSION + 1, sortKey: "x" }),
    );
    expect(loadTreeView()).toEqual({});
    expect(localStorage.getItem(TREE_VIEW_KEY)).toBeNull();
  });

  it("drops an unparsable record instead of throwing", () => {
    localStorage.setItem(TREE_VIEW_KEY, "{oops");
    expect(loadTreeView()).toEqual({});
  });
});

describe("tree view persistence", () => {
  beforeEach(() => {
    localStorage.clear();
    clearTreeView();
    resetTreeSort();
    resetTreeFilter();
  });

  it("restores the sort key, the hidden states and the group mode on a reload", () => {
    act(() => setTreeSortKey("modified_desc"));
    act(() => setHiddenStates(["completed"]));
    act(() => setGroupMode("full"));
    // a reload is a fresh module: the seeded state is what storage holds
    vi.resetModules();
    return import("./treeViewStore").then(async () => {
      const sort = await import("./treeSortStore");
      const filter = await import("./treeFilterStore");
      expect(renderHook(() => sort.useTreeSort()).result.current.key).toBe("modified_desc");
      expect(renderHook(() => filter.useTreeFilter()).result.current).toEqual({
        hiddenStates: ["completed"],
        groupMode: "full",
      });
    });
  });

  it("clears the stored slice when a control resets", () => {
    act(() => setTreeSortKey("name_asc"));
    act(() => setGroupMode("flat"));
    act(() => resetTreeSort());
    expect(loadTreeView().sortKey).toBeUndefined();
    expect(loadTreeView().groupMode).toBe("flat");
    act(() => resetTreeFilter());
    expect(localStorage.getItem(TREE_VIEW_KEY)).toBeNull();
  });

  it("ignores a stored value the control no longer offers", () => {
    localStorage.setItem(
      TREE_VIEW_KEY,
      JSON.stringify({ version: TREE_VIEW_VERSION, sortKey: "bogus", groupMode: "sideways" }),
    );
    vi.resetModules();
    return Promise.all([import("./treeSortStore"), import("./treeFilterStore")]).then(
      ([sort, filter]) => {
        expect(renderHook(() => sort.useTreeSort()).result.current.key).toBe("created_desc");
        expect(renderHook(() => filter.useTreeFilter()).result.current).toEqual({
          hiddenStates: [],
          groupMode: "type",
        });
      },
    );
  });

  it("drops a malformed hidden-state slice", () => {
    localStorage.setItem(
      TREE_VIEW_KEY,
      JSON.stringify({ version: TREE_VIEW_VERSION, hiddenStates: { nope: true } }),
    );
    vi.resetModules();
    return import("./treeFilterStore").then((filter) => {
      expect(renderHook(() => filter.useTreeFilter()).result.current.hiddenStates).toEqual([]);
    });
  });
});
