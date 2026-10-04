// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { DEFAULT_FIND_OPTIONS } from "./findInDoc";
import {
  closeFind,
  openFind,
  resetFindOptions,
  resetFindQuery,
  setFindQuery,
  toggleFindOption,
  useFindInDoc,
} from "./findInDocStore";

afterEach(() => {
  closeFind();
  setFindQuery("");
  resetFindOptions();
});

describe("findInDocStore", () => {
  it("clears the query while the options and the open state survive", () => {
    const { result } = renderHook(() => useFindInDoc());
    act(() => {
      openFind();
      toggleFindOption("caseSensitive");
      setFindQuery("tokens");
    });
    expect(result.current.query).toBe("tokens");
    expect(result.current.open).toBe(true);
    expect(result.current.options).toEqual({ ...DEFAULT_FIND_OPTIONS, caseSensitive: true });

    act(() => resetFindQuery());
    expect(result.current.query).toBe("");
    expect(result.current.open).toBe(true);
    expect(result.current.options).toEqual({ ...DEFAULT_FIND_OPTIONS, caseSensitive: true });
  });
});
