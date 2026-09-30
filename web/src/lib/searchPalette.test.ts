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
  queryTerms,
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
    state = paletteReducer(state, { type: "objective", value: "viewer" });
    state = paletteReducer(state, { type: "category", value: "bug" });
    state = paletteReducer(state, { type: "from", value: "2026-01-01" });
    state = paletteReducer(state, { type: "to", value: "2026-12-31" });
    expect(state.filters).toEqual({
      kind: "wiki",
      objective: "viewer",
      category: "bug",
      from: "2026-01-01",
      to: "2026-12-31",
    });
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

  it("keeps the filters while clearing results, so a short query does not drop them", () => {
    const state = paletteReducer(
      {
        ...initialPaletteState,
        query: "t",
        filters: { kind: "wiki", objective: "", category: "", from: "", to: "" },
      },
      { type: "cleared" },
    );
    expect(state.filters.kind).toBe("wiki");
    expect(state.status).toBe("idle");
  });

  it("drops every filter on resetFilters but keeps the results", () => {
    const state = paletteReducer(
      {
        ...initialPaletteState,
        results: [result({})],
        total: 1,
        status: "ready",
        filters: { kind: "wiki", objective: "viewer", category: "bug", from: "a", to: "b" },
      },
      { type: "resetFilters" },
    );
    expect(state.filters).toEqual(EMPTY_FILTERS);
    expect(state.results).toHaveLength(1);
    expect(state.status).toBe("ready");
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

  it("includes the objective filter", () => {
    const url = buildSearchUrl({ q: "tokens", objective: "viewer", limit: 5 });
    const params = new URLSearchParams(url.split("?")[1]);
    expect(params.get("objective")).toBe("viewer");
  });

  it("includes the category filter", () => {
    const url = buildSearchUrl({ q: "tokens", category: "bug", limit: 5 });
    const params = new URLSearchParams(url.split("?")[1]);
    expect(params.get("category")).toBe("bug");
  });
});

describe("resultRoute", () => {
  it("maps corpus paths to the document detail route", () => {
    expect(resultRoute("context/wiki/alpha.md")).toBe("/docs/context/wiki/alpha.md");
  });
});

describe("resultTitle", () => {
  it("prefers the frontmatter title, else the formatted filename", () => {
    expect(resultTitle(result({ title: "Alpha" }))).toBe("Alpha");
    expect(resultTitle(result({ title: undefined }))).toBe("Alpha");
  });
});

describe("queryTerms", () => {
  it("splits on whitespace like the server Snippet does", () => {
    expect(queryTerms("  Alpha  Tokens  ")).toEqual(["tokens", "alpha"]);
    expect(queryTerms("")).toEqual([]);
    expect(queryTerms("   ")).toEqual([]);
  });

  it("sorts the longest term first so an overlapping term matches whole", () => {
    expect(queryTerms("tok token")).toEqual(["token", "tok"]);
  });
});

describe("highlightSegments", () => {
  it("flags case-insensitive matches", () => {
    const segs = highlightSegments("Tokens and tokens", "tokens");
    expect(segs.filter((s) => s.match).map((s) => s.text)).toEqual(["Tokens", "tokens"]);
  });

  it("flags each term of a multi-word query, as the server snippet is built", () => {
    const segs = highlightSegments("alpha handles tokens today", "tokens alpha");
    expect(segs.filter((s) => s.match).map((s) => s.text)).toEqual(["alpha", "tokens"]);
  });

  it("escapes regex metacharacters in a term", () => {
    expect(highlightSegments("cost is a.b or a.b.c", "a.b")).toEqual([
      { text: "cost is ", match: false },
      { text: "a.b", match: true },
      { text: " or ", match: false },
      { text: "a.b", match: true },
      { text: ".c", match: false },
    ]);
  });

  it("keeps the surrounding text and order intact", () => {
    expect(highlightSegments("a alpha b beta c", "alpha beta")).toEqual([
      { text: "a ", match: false },
      { text: "alpha", match: true },
      { text: " b ", match: false },
      { text: "beta", match: true },
      { text: " c", match: false },
    ]);
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
