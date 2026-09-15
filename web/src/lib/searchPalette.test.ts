// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { buildSearchUrl, type SearchResult } from "./api";
import {
  EMPTY_FILTERS,
  highlightSegments,
  initialPaletteState,
  isSearchable,
  paletteReducer,
  resultRoute,
  resultTitle,
  useDebouncedValue,
  type PaletteState,
} from "./searchPalette";

function result(patch: Partial<SearchResult>): SearchResult {
  return { path: "context/wiki/alpha.md", score: 1, snippet: "alpha tokens", ...patch };
}

describe("paletteReducer", () => {
  it("updates the query without touching results", () => {
    const state = paletteReducer(
      { ...initialPaletteState, results: [result({})], total: 1 },
      { type: "query", value: "tok" },
    );
    expect(state.query).toBe("tok");
    expect(state.results).toHaveLength(1);
  });

  it("composes filters independently", () => {
    let state = paletteReducer(initialPaletteState, { type: "kind", value: "wiki" });
    state = paletteReducer(state, { type: "from", value: "2026-01-01" });
    state = paletteReducer(state, { type: "to", value: "2026-12-31" });
    expect(state.filters).toEqual({ kind: "wiki", from: "2026-01-01", to: "2026-12-31" });
    state = paletteReducer(state, { type: "resetFilters" });
    expect(state.filters).toEqual(EMPTY_FILTERS);
  });

  it("runs load → loaded → cleared lifecycle", () => {
    const base: PaletteState = { ...initialPaletteState, query: "tokens" };
    let state = paletteReducer(base, { type: "load" });
    expect(state.status).toBe("loading");
    state = paletteReducer(state, { type: "loaded", results: [result({})], total: 1 });
    expect(state.status).toBe("ready");
    expect(state.total).toBe(1);
    state = paletteReducer(state, { type: "cleared" });
    expect(state.status).toBe("idle");
    expect(state.results).toHaveLength(0);
    expect(state.query).toBe("tokens");
  });

  it("records failures and drops stale results", () => {
    const state = paletteReducer(
      { ...initialPaletteState, status: "loading", results: [result({})] },
      { type: "failed", error: "boom" },
    );
    expect(state.status).toBe("error");
    expect(state.error).toBe("boom");
    expect(state.results).toHaveLength(0);
  });
});

describe("isSearchable", () => {
  it("requires the minimum trimmed length", () => {
    expect(isSearchable("")).toBe(false);
    expect(isSearchable(" t ")).toBe(false);
    expect(isSearchable(" to ")).toBe(true);
  });
});

describe("buildSearchUrl", () => {
  it("omits empty filters and encodes the query", () => {
    const url = buildSearchUrl({ q: "hello world", kind: "", from: "", to: "", limit: 20 });
    expect(url).toBe("/api/search?q=hello+world&limit=20");
  });

  it("includes kind and date bounds", () => {
    const url = buildSearchUrl({
      q: "tokens",
      kind: "wiki",
      from: "2026-01-01",
      to: "2026-12-31",
      limit: 5,
    });
    const params = new URLSearchParams(url.split("?")[1]);
    expect(params.get("q")).toBe("tokens");
    expect(params.get("kind")).toBe("wiki");
    expect(params.get("from")).toBe("2026-01-01");
    expect(params.get("to")).toBe("2026-12-31");
    expect(params.get("limit")).toBe("5");
  });
});

describe("resultRoute", () => {
  it("maps corpus paths to the document detail route", () => {
    expect(resultRoute("context/wiki/alpha.md")).toBe("/docs/context/wiki/alpha.md");
  });
});

describe("resultTitle", () => {
  it("prefers the frontmatter title, else the basename", () => {
    expect(resultTitle(result({ title: "Alpha" }))).toBe("Alpha");
    expect(resultTitle(result({ title: undefined }))).toBe("alpha");
  });
});

describe("highlightSegments", () => {
  it("flags case-insensitive matches", () => {
    const segs = highlightSegments("Tokens and tokens", "tokens");
    expect(segs.filter((s) => s.match).map((s) => s.text)).toEqual(["Tokens", "tokens"]);
  });

  it("returns a single non-match segment without a query", () => {
    expect(highlightSegments("plain text", "  ")).toEqual([{ text: "plain text", match: false }]);
  });

  it("handles no matches", () => {
    expect(highlightSegments("plain text", "zzz")).toEqual([{ text: "plain text", match: false }]);
  });
});

describe("useDebouncedValue", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("emits only the last value after the delay", () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(({ value }) => useDebouncedValue(value, 200), {
      initialProps: { value: "a" },
    });
    expect(result.current).toBe("a");

    rerender({ value: "ab" });
    rerender({ value: "abc" });
    expect(result.current).toBe("a");

    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current).toBe("abc");
  });
});
