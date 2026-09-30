import { useEffect, useState } from "react";
import type { SearchResult } from "./api";
import { KIND_ORDER, type EntryKind } from "./kinds";
import { displayTitle } from "./titles";

/** Minimum trimmed query length before a search request is issued. */
export const MIN_QUERY_LENGTH = 2;

/** Server-side result cap passed as the `limit` param. */
export const SEARCH_LIMIT = 20;

/** Kind options shown in the palette; canvas is not searchable. */
export const KIND_OPTIONS: EntryKind[] = [...KIND_ORDER];

export interface SearchFilters {
  /** exact frontmatter kind; "" = any */
  kind: string;
  /** exact frontmatter objective; "" = any */
  objective: string;
  /** exact frontmatter category; "" = any */
  category: string;
  /** inclusive created-date lower bound (YYYY-MM-DD); "" = none */
  from: string;
  /** inclusive created-date upper bound (YYYY-MM-DD); "" = none */
  to: string;
}

export const EMPTY_FILTERS: SearchFilters = {
  kind: "",
  objective: "",
  category: "",
  from: "",
  to: "",
};

export type PaletteStatus = "idle" | "loading" | "ready" | "error";

export interface PaletteState {
  query: string;
  filters: SearchFilters;
  status: PaletteStatus;
  results: SearchResult[];
  total: number;
  error: string | null;
}

export const initialPaletteState: PaletteState = {
  query: "",
  filters: { ...EMPTY_FILTERS },
  status: "idle",
  results: [],
  total: 0,
  error: null,
};

/**
 * A cleared palette drops its results and status but keeps the query (the input
 * owns it) and the filters the user is still typing against; closing the
 * palette dispatches `resetFilters` so a reopen starts unfiltered.
 */
export type PaletteAction =
  | { type: "query"; value: string }
  | { type: "kind"; value: string }
  | { type: "objective"; value: string }
  | { type: "category"; value: string }
  | { type: "from"; value: string }
  | { type: "to"; value: string }
  | { type: "resetFilters" }
  | { type: "load" }
  | { type: "loaded"; results: SearchResult[]; total: number }
  | { type: "failed"; error: string }
  | { type: "cleared" };

/** Pure palette state machine: query/filter edits + async result lifecycle. */
export function paletteReducer(state: PaletteState, action: PaletteAction): PaletteState {
  switch (action.type) {
    case "query":
      return { ...state, query: action.value };
    case "kind":
      return { ...state, filters: { ...state.filters, kind: action.value } };
    case "objective":
      return { ...state, filters: { ...state.filters, objective: action.value } };
    case "category":
      return { ...state, filters: { ...state.filters, category: action.value } };
    case "from":
      return { ...state, filters: { ...state.filters, from: action.value } };
    case "to":
      return { ...state, filters: { ...state.filters, to: action.value } };
    case "resetFilters":
      return { ...state, filters: { ...EMPTY_FILTERS } };
    case "load":
      return { ...state, status: "loading", error: null };
    case "loaded":
      return {
        ...state,
        status: "ready",
        results: action.results,
        total: action.total,
        error: null,
      };
    case "failed":
      return { ...state, status: "error", results: [], total: 0, error: action.error };
    case "cleared":
      // results only: the query stays (the input owns it) and so do the filters,
      // which the user may still be typing against. `resetFilters` drops them on
      // close, so a reopened palette never filters silently.
      return { ...state, status: "idle", results: [], total: 0, error: null };
  }
}

/** Whether the query is long enough to hit /api/search. */
export function isSearchable(query: string): boolean {
  return query.trim().length >= MIN_QUERY_LENGTH;
}

/** Document-detail browser route for a corpus path. */
export function resultRoute(path: string): string {
  return `/docs/${path}`;
}

export interface HighlightSegment {
  text: string;
  match: boolean;
}

/** Escape a literal term for use inside a RegExp. */
function escapeRegExp(term: string): string {
  return term.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Query terms exactly as the server tokenises them: `internal/search.Snippet`
 * splits on whitespace after lowercasing, and the snippet is a window around the
 * first *individual* term. Longest first, so a term that contains another is
 * still matched whole.
 */
export function queryTerms(query: string): string[] {
  return query
    .trim()
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean)
    .sort((a, b) => b.length - a.length);
}

/**
 * Split text into segments, flagging case-insensitive occurrences of any query
 * term. Matching per term (not per phrase) is what the server snippet is built
 * from, so a multi-word hit shows why it matched.
 */
export function highlightSegments(text: string, query: string): HighlightSegment[] {
  const pattern = queryTerms(query).map(escapeRegExp).join("|");
  if (!pattern) return [{ text, match: false }];
  const matches = [...text.matchAll(new RegExp(pattern, "gi"))];
  if (matches.length === 0) return [{ text, match: false }];
  const segments: HighlightSegment[] = [];
  let cursor = 0;
  for (const match of matches) {
    const at = match.index ?? 0;
    if (at > cursor) segments.push({ text: text.slice(cursor, at), match: false });
    segments.push({ text: match[0], match: true });
    cursor = at + match[0].length;
  }
  if (cursor < text.length) segments.push({ text: text.slice(cursor), match: false });
  return segments;
}

/** Display title for a result via the shared cascade (title → formatted path). */
export function resultTitle(r: SearchResult): string {
  return displayTitle({ title: r.title, path: r.path });
}

/** Debounce a value by delayMs (used to pace /api/search calls). */
export function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(id);
  }, [value, delayMs]);
  return debounced;
}
