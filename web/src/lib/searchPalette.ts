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
  /** inclusive created-date lower bound (YYYY-MM-DD); "" = none */
  from: string;
  /** inclusive created-date upper bound (YYYY-MM-DD); "" = none */
  to: string;
}

export const EMPTY_FILTERS: SearchFilters = { kind: "", objective: "", from: "", to: "" };

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

export type PaletteAction =
  | { type: "query"; value: string }
  | { type: "kind"; value: string }
  | { type: "objective"; value: string }
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

/** Split text into segments, flagging case-insensitive occurrences of query. */
export function highlightSegments(text: string, query: string): HighlightSegment[] {
  const term = query.trim();
  if (!term) return [{ text, match: false }];
  const segments: HighlightSegment[] = [];
  const lower = text.toLowerCase();
  const needle = term.toLowerCase();
  let i = 0;
  while (i < text.length) {
    const at = lower.indexOf(needle, i);
    if (at === -1) {
      segments.push({ text: text.slice(i), match: false });
      break;
    }
    if (at > i) segments.push({ text: text.slice(i, at), match: false });
    segments.push({ text: text.slice(at, at + needle.length), match: true });
    i = at + needle.length;
  }
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
