// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { resetTreeFilter, setHiddenStates, toggleGrouped, useTreeFilter } from "./treeFilterStore";

describe("treeFilterStore", () => {
  // module-level store: reset so each case starts from DEFAULT
  beforeEach(() => {
    resetTreeFilter();
  });

  it("defaults to nothing hidden and grouped false", () => {
    const { result } = renderHook(() => useTreeFilter());
    expect(result.current.hiddenStates).toEqual([]);
    expect(result.current.grouped).toBe(false);
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

  it("toggles grouped without touching the hidden states", () => {
    const { result } = renderHook(() => useTreeFilter());
    act(() => setHiddenStates(["completed"]));
    act(() => toggleGrouped());
    expect(result.current.grouped).toBe(true);
    expect(result.current.hiddenStates).toEqual(["completed"]);
  });

  it("resets both fields to their defaults", () => {
    const { result } = renderHook(() => useTreeFilter());
    act(() => setHiddenStates(["completed"]));
    act(() => toggleGrouped());
    expect(result.current.hiddenStates).toEqual(["completed"]);
    expect(result.current.grouped).toBe(true);
    act(() => resetTreeFilter());
    expect(result.current.hiddenStates).toEqual([]);
    expect(result.current.grouped).toBe(false);
  });
});
