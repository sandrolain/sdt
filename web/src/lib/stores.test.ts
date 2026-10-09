// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { act, renderHook } from "@testing-library/react";
import {
  consumeSectionRequest,
  getSectionRequest,
  requestSection,
  resetSectionRequest,
} from "./sectionRequests";
import { requestTreeFocus, resetTreeFocusRequests, useTreeFocusRequests } from "./treeFocus";
import { isPinned, resetPinnedDocs, togglePin, usePinnedDocs } from "./pinnedDocs";
import { resetTreeSort, setTreeSortKey, useTreeSort } from "./treeSortStore";
import { clearTreeView } from "./treeViewStore";

afterEach(() => {
  localStorage.clear();
  resetSectionRequest();
  resetTreeFocusRequests();
  resetPinnedDocs();
  clearTreeView();
  resetTreeSort();
});

describe("sectionRequests", () => {
  it("bumps the nonce per request and clears on consume", () => {
    resetSectionRequest();
    requestSection("a.md", "H");
    const first = getSectionRequest()!;
    expect(first.nonce).toBe(1);
    requestSection("b.md", "H2");
    const second = getSectionRequest()!;
    expect(second.nonce).toBe(2);
    // a stale consume is ignored
    consumeSectionRequest(first);
    expect(getSectionRequest()).toBe(second);
    consumeSectionRequest(second);
    expect(getSectionRequest()).toBeNull();
  });
});

describe("treeFocus", () => {
  it("counts focus requests", () => {
    resetTreeFocusRequests();
    const { result } = renderHook(() => useTreeFocusRequests());
    expect(result.current).toBe(0);
    act(() => requestTreeFocus());
    act(() => requestTreeFocus());
    expect(result.current).toBe(2);
  });
});

describe("pinnedDocs", () => {
  it("toggles pins newest-first and persists", () => {
    resetPinnedDocs();
    const { result } = renderHook(() => usePinnedDocs());
    act(() => togglePin("a"));
    act(() => togglePin("b"));
    expect(result.current).toEqual(["b", "a"]);
    expect(isPinned("a")).toBe(true);
    act(() => togglePin("a"));
    expect(result.current).toEqual(["b"]);
    expect(isPinned("a")).toBe(false);
  });
});

describe("treeSortStore", () => {
  it("defaults and sets the sort key", () => {
    clearTreeView();
    resetTreeSort();
    const { result } = renderHook(() => useTreeSort());
    expect(result.current.key).toBe("created_desc");
    act(() => setTreeSortKey("title_asc"));
    expect(result.current.key).toBe("title_asc");
  });
});
