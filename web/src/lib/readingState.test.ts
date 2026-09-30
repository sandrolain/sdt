// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import {
  clampScrollTop,
  restoreScrollOffset,
  scrollToHeading,
  clearReadingState,
  flushReading,
  loadReadingState,
  READING_KEY,
  RECENT_CAP,
  recordReading,
  readingPosition,
  useRecentDocuments,
  WRITE_THROTTLE_MS,
} from "./readingState";

describe("readingState", () => {
  beforeEach(() => {
    localStorage.clear();
    clearReadingState();
  });

  it("stores a position per document and returns it", () => {
    recordReading("a.md", 420.4, "Phase 2", 0);
    expect(readingPosition("a.md")).toEqual({ scrollTop: 420, headingId: "Phase 2" });
    expect(readingPosition("missing.md")).toBeNull();
  });

  it("keeps a negative offset at the top", () => {
    recordReading("a.md", -50, null, 0);
    expect(readingPosition("a.md")?.scrollTop).toBe(0);
  });

  it("orders the recent list newest first without duplicates", () => {
    recordReading("a.md", 10, null, 0);
    recordReading("b.md", 20, null, 0);
    recordReading("a.md", 30, null, 0);
    const { result } = renderHook(() => useRecentDocuments());
    expect(result.current).toEqual(["a.md", "b.md"]);
  });

  it("caps the recent list", () => {
    for (let i = 0; i < RECENT_CAP + 5; i++) recordReading(`doc${i}.md`, i, null, 0);
    const { result } = renderHook(() => useRecentDocuments());
    expect(result.current).toHaveLength(RECENT_CAP);
    expect(result.current[0]).toBe(`doc${RECENT_CAP + 4}.md`);
  });

  it("throttles writes while scrolling and flushes on demand", () => {
    recordReading("a.md", 10, null, 1_000);
    const written = () => localStorage.getItem(READING_KEY);
    expect(written()).toContain("10");
    // inside the throttle window the in-memory state moves but storage does not
    recordReading("a.md", 900, null, 1_000 + WRITE_THROTTLE_MS - 1);
    expect(written()).toContain("10");
    expect(readingPosition("a.md")?.scrollTop).toBe(900);
    flushReading();
    expect(written()).toContain("900");
  });

  it("survives a reload and drops a malformed record", () => {
    recordReading("a.md", 33, "Design", 0);
    flushReading();
    expect(loadReadingState().positions["a.md"]).toEqual({ scrollTop: 33, headingId: "Design" });

    localStorage.setItem(READING_KEY, "{ not json");
    expect(loadReadingState()).toEqual({ positions: {}, recent: [] });
    expect(localStorage.getItem(READING_KEY)).toBeNull();

    localStorage.setItem(
      READING_KEY,
      JSON.stringify({ positions: { "a.md": { scrollTop: "x" } } }),
    );
    expect(loadReadingState().positions).toEqual({});
  });

  it("notifies subscribers when a new document is read", () => {
    recordReading("a.md", 10, null, 0);
    const { result } = renderHook(() => useRecentDocuments().length);
    expect(result.current).toBe(1);
    act(() => recordReading("b.md", 20, null, 0));
    expect(result.current).toBe(2);
  });
});

describe("clampScrollTop", () => {
  it("keeps an offset inside the scrollable range", () => {
    expect(clampScrollTop(300, 2000, 800)).toBe(300);
  });

  it("clamps past the end when the document shrank", () => {
    expect(clampScrollTop(1800, 1000, 400)).toBe(600);
  });

  it("clamps a negative or non-scrollable document to the top", () => {
    expect(clampScrollTop(-20, 1000, 400)).toBe(0);
    expect(clampScrollTop(500, 300, 400)).toBe(0);
  });
});

describe("restore", () => {
  function doc(scrollHeight: number, clientHeight: number): HTMLElement {
    const el = document.createElement("div");
    el.innerHTML = '<h2 data-heading="Findings">Findings</h2><h3 data-heading="Detail">Detail</h3>';
    Object.defineProperty(el, "scrollHeight", { value: scrollHeight, configurable: true });
    Object.defineProperty(el, "clientHeight", { value: clientHeight, configurable: true });
    document.body.appendChild(el);
    return el;
  }

  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("restores the offset when the document can still scroll there", () => {
    recordReading("a.md", 900, "Findings", 0);
    const el = doc(3000, 600);
    restoreScrollOffset(el, "a.md");
    expect(el.scrollTop).toBe(900);
  });

  it("falls back to the recorded heading when the offset is gone", () => {
    recordReading("a.md", 0, "Detail", 0);
    const el = doc(3000, 600);
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    restoreScrollOffset(el, "a.md");
    expect(scrollIntoView).toHaveBeenCalled();
  });

  it("falls back to the heading when the document shrank past the offset", () => {
    recordReading("a.md", 5000, "Findings", 0);
    const el = doc(500, 600); // nothing to scroll: the offset is stale
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    restoreScrollOffset(el, "a.md");
    expect(el.scrollTop).toBe(0);
    expect(scrollIntoView).toHaveBeenCalled();
  });

  it("reports a heading that no longer exists", () => {
    recordReading("a.md", 0, "Removed section", 0);
    const el = doc(3000, 600);
    expect(scrollToHeading(el, "Removed section")).toBe(false);
    expect(scrollToHeading(el, "Findings")).toBe(true);
    expect(scrollToHeading(el, null)).toBe(false);
  });
});
