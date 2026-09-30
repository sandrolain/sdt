// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";
import { act, renderHook } from "@testing-library/react";
import {
  clampScrollTop,
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
