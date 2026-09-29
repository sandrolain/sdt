// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { resetTreeFilter, setGroupMode, setHiddenStates, useTreeFilter } from "./treeFilterStore";

describe("treeFilterStore", () => {
  // module-level store: reset so each case starts from DEFAULT
  beforeEach(() => {
    resetTreeFilter();
  });

  it("defaults to nothing hidden and the By type grouping", () => {
    const { result } = renderHook(() => useTreeFilter());
    expect(result.current.hiddenStates).toEqual([]);
    expect(result.current.groupMode).toBe("type");
  });

  it("replaces the hidden states", () => {
    const { result } = renderHook(() => useTreeFilter());
    act(() => setHiddenStates(["completed"]));
    expect(result.current.hiddenStates).toEqual(["completed"]);
    act(() => setHiddenStates(["completed", "archived"]));
    expect(result.current.hiddenStates).toEqual(["completed", "archived"]);
    act(() => setHiddenStates([]));
    expect(result.current.hiddenStates).toEqual([]);
  });

  it("replaces the grouping mode without touching the hidden states", () => {
    const { result } = renderHook(() => useTreeFilter());
    act(() => setHiddenStates(["completed"]));
    act(() => setGroupMode("flat"));
    expect(result.current.groupMode).toBe("flat");
    expect(result.current.hiddenStates).toEqual(["completed"]);
    act(() => setGroupMode("full"));
    expect(result.current.groupMode).toBe("full");
  });

  it("resets both fields to their defaults", () => {
    const { result } = renderHook(() => useTreeFilter());
    act(() => setHiddenStates(["completed"]));
    act(() => setGroupMode("flat"));
    expect(result.current.hiddenStates).toEqual(["completed"]);
    expect(result.current.groupMode).toBe("flat");
    act(() => resetTreeFilter());
    expect(result.current.hiddenStates).toEqual([]);
    expect(result.current.groupMode).toBe("type");
  });
});
