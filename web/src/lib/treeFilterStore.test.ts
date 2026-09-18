// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { resetTreeFilter, toggleHideCompleted, useTreeFilter } from "./treeFilterStore";

describe("treeFilterStore", () => {
  it("defaults to hideCompleted false", () => {
    const { result } = renderHook(() => useTreeFilter());
    expect(result.current.hideCompleted).toBe(false);
  });

  it("toggles hideCompleted", () => {
    const { result } = renderHook(() => useTreeFilter());
    act(() => toggleHideCompleted());
    expect(result.current.hideCompleted).toBe(true);
    act(() => toggleHideCompleted());
    expect(result.current.hideCompleted).toBe(false);
  });

  it("resets to false", () => {
    const { result } = renderHook(() => useTreeFilter());
    act(() => toggleHideCompleted());
    expect(result.current.hideCompleted).toBe(true);
    act(() => resetTreeFilter());
    expect(result.current.hideCompleted).toBe(false);
  });
});
