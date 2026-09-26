// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";
import { renderHook, act } from "@testing-library/react";
import {
  resetTreeFilter,
  toggleGrouped,
  toggleHideCompleted,
  useTreeFilter,
} from "./treeFilterStore";

describe("treeFilterStore", () => {
  // module-level store: reset so each case starts from DEFAULT
  beforeEach(() => {
    resetTreeFilter();
  });

  it("defaults to hideCompleted false and grouped true", () => {
    const { result } = renderHook(() => useTreeFilter());
    expect(result.current.hideCompleted).toBe(false);
    expect(result.current.grouped).toBe(true);
  });

  it("toggles hideCompleted", () => {
    const { result } = renderHook(() => useTreeFilter());
    act(() => toggleHideCompleted());
    expect(result.current.hideCompleted).toBe(true);
    act(() => toggleHideCompleted());
    expect(result.current.hideCompleted).toBe(false);
  });

  it("toggles grouped without touching hideCompleted", () => {
    const { result } = renderHook(() => useTreeFilter());
    act(() => toggleHideCompleted());
    act(() => toggleGrouped());
    expect(result.current.grouped).toBe(false);
    expect(result.current.hideCompleted).toBe(true);
  });

  it("resets to false", () => {
    const { result } = renderHook(() => useTreeFilter());
    act(() => toggleHideCompleted());
    act(() => toggleGrouped());
    expect(result.current.hideCompleted).toBe(true);
    expect(result.current.grouped).toBe(false);
    act(() => resetTreeFilter());
    expect(result.current.hideCompleted).toBe(false);
    expect(result.current.grouped).toBe(true);
  });
});
