import { useEffect, useState } from "react";
import type { SearchResult } from "./api";
import { KIND_ORDER, type EntryKind } from "./kinds";
import { displayTitle } from "./titles";

/** Results per request; "show more" asks for one more page on top of what is shown. */
export const SEARCH_LIMIT = 20;

/** Kind options shown in the palette; canvas is not searchable. */
export const KIND_OPTIONS: EntryKind[] = [...KIND_ORDER];

export interface SearchFilters {
  /** exact frontmatter kind; "" = any */
  kind: string;
  /** exact frontmatter objective (from the corpus register); "" = any */
  objective: string;
  /** exact frontmatter status; "" = any */
  status: string;
  /** exact frontmatter topic (from the corpus register); "" = any */
  topic: string;
  /** exact frontmatter category (from the corpus register); "" = any */
  category: string;
  /** inclusive created-date lower bound (YYYY-MM-DD); "" = none */
  from: string;
  /** inclusive created-date upper bound (YYYY-MM-DD); "" = none */
  to: string;
}

export const EMPTY_FILTERS: SearchFilters = {
  kind: "",
  objective: "",
  status: "",
  topic: "",
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
  /** number of results requested so far (SEARCH_LIMIT * pages) */
  limit: number;
  total: number;
  /** true when the server answered with substring matches, not lexical hits */
  partial: boolean;
  error: string | null;
}

export const initialPaletteState: PaletteState = {
  query: "",
  filters: { ...EMPTY_FILTERS },
  status: "idle",
  results: [],
  limit: SEARCH_LIMIT,
  total: 0,
  partial: false,
  error: null,
};

export type PaletteAction =
  | { type: "query"; value: string }
  | { type: "kind"; value: string }
  | { type: "objective"; value: string }
  | { type: "status"; value: string }
  | { type: "topic"; value: string }
  | { type: "category"; value: string }
  | { type: "from"; value: string }
  | { type: "to"; value: string }
  | { type: "resetFilters" }
  | { type: "more" }
  | { type: "load" }
  | { type: "loaded"; results: SearchResult[]; total: number; partial?: boolean }
  | { type: "failed"; error: string }
  | { type: "cleared" };

/** Pure palette state machine: query/filter edits + async result lifecycle. */
export function paletteReducer(state: PaletteState, action: PaletteAction): PaletteState {
  switch (action.type) {
    case "query":
      // a new query restarts paging
      return { ...state, query: action.value, limit: SEARCH_LIMIT };
    case "kind":
      return withFilter(state, { kind: action.value });
    case "objective":
      return withFilter(state, { objective: action.value });
    case "status":
      return withFilter(state, { status: action.value });
    case "topic":
      return withFilter(state, { topic: action.value });
    case "category":
      return withFilter(state, { category: action.value });
    case "from":
      return withFilter(state, { from: action.value });
    case "to":
      return withFilter(state, { to: action.value });
    case "resetFilters":
      return { ...state, filters: { ...EMPTY_FILTERS } };
    case "more":
      return { ...state, limit: state.limit + SEARCH_LIMIT };
    case "load":
      return { ...state, status: "loading", error: null };
    case "loaded":
      return {
        ...state,
        status: "ready",
        results: action.results,
        total: action.total,
        partial: action.partial ?? false,
        error: null,
      };
    case "failed":
      return {
        ...state,
        status: "error",
        results: [],
        total: 0,
        partial: false,
        error: action.error,
      };
    case "cleared":
      // results only: the query stays (the input owns it) and so do the filters,
      // which the user may still be typing against. `resetFilters` drops them on
      // close, so a reopened palette never filters silently.
      return {
        ...state,
        status: "idle",
        results: [],
        total: 0,
        partial: false,
        error: null,
        limit: SEARCH_LIMIT,
      };
  }
}

/** Apply a filter patch and restart paging. */
function withFilter(state: PaletteState, patch: Partial<SearchFilters>): PaletteState {
  return { ...state, filters: { ...state.filters, ...patch }, limit: SEARCH_LIMIT };
}

/**
 * Document-detail browser route for a corpus path. A matched section anchor
 * becomes the fragment, so the hit deep-links to where it matched.
 */
export function resultRoute(path: string, section?: string): string {
  return section ? `/docs/${path}#${section}` : `/docs/${path}`;
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

/** Whether more results exist beyond what has been requested. */
export function hasMore(state: PaletteState): boolean {
  return state.results.length < state.total;
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
