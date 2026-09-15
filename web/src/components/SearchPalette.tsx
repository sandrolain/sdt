import { useEffect, useReducer } from "react";
import { useNavigate } from "react-router-dom";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "cmdk";
import { fetchSearch } from "../lib/api";
import { SkeletonLines } from "./Skeleton";
import {
  isSearchable,
  KIND_OPTIONS,
  MIN_QUERY_LENGTH,
  paletteReducer,
  initialPaletteState,
  highlightSegments,
  resultRoute,
  resultTitle,
  SEARCH_LIMIT,
  useDebouncedValue,
} from "../lib/searchPalette";

interface SearchPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Modal command palette: debounced /api/search with kind/date filters. */
export function SearchPalette({ open, onOpenChange }: SearchPaletteProps) {
  const [state, dispatch] = useReducer(paletteReducer, initialPaletteState);
  const debouncedQuery = useDebouncedValue(state.query, 200);
  const navigate = useNavigate();
  const { kind, from, to } = state.filters;

  useEffect(() => {
    if (!open) dispatch({ type: "cleared" });
  }, [open]);

  useEffect(() => {
    if (!isSearchable(debouncedQuery)) {
      dispatch({ type: "cleared" });
      return;
    }
    let alive = true;
    dispatch({ type: "load" });
    fetchSearch({ q: debouncedQuery.trim(), kind, from, to, limit: SEARCH_LIMIT })
      .then((res) => {
        if (alive) dispatch({ type: "loaded", results: res.results ?? [], total: res.total });
      })
      .catch((err: unknown) => {
        if (alive)
          dispatch({ type: "failed", error: err instanceof Error ? err.message : String(err) });
      });
    return () => {
      alive = false;
    };
  }, [debouncedQuery, kind, from, to]);

  const select = (path: string) => {
    onOpenChange(false);
    navigate(resultRoute(path));
  };

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      label="Search the corpus"
      className="search-palette"
      overlayClassName="search-palette__overlay"
      contentClassName="search-palette__content"
      shouldFilter={false}
      loop
    >
      <CommandInput
        className="search-palette__input"
        placeholder="Search titles, summaries and bodies…"
        value={state.query}
        onValueChange={(value) => dispatch({ type: "query", value })}
        aria-label="Search query"
      />
      <div
        className="search-filters"
        aria-label="Search filters"
        onKeyDown={(e) => {
          if (e.key.startsWith("Arrow")) e.stopPropagation();
        }}
      >
        <label className="search-filters__field">
          <span className="search-filters__label">Kind</span>
          <select
            className="search-filters__control"
            value={kind}
            onChange={(e) => dispatch({ type: "kind", value: e.target.value })}
          >
            <option value="">any</option>
            {KIND_OPTIONS.map((k) => (
              <option key={k} value={k}>
                {k}
              </option>
            ))}
          </select>
        </label>
        <label className="search-filters__field">
          <span className="search-filters__label">From</span>
          <input
            type="date"
            className="search-filters__control"
            value={from}
            max={to || undefined}
            onChange={(e) => dispatch({ type: "from", value: e.target.value })}
          />
        </label>
        <label className="search-filters__field">
          <span className="search-filters__label">To</span>
          <input
            type="date"
            className="search-filters__control"
            value={to}
            min={from || undefined}
            onChange={(e) => dispatch({ type: "to", value: e.target.value })}
          />
        </label>
      </div>
      <CommandList className="search-palette__list">
        <PaletteEmpty
          status={state.status}
          error={state.error}
          searchable={isSearchable(debouncedQuery)}
        />
        {state.results.length > 0 && (
          <CommandGroup
            className="search-palette__group"
            heading={`${state.total} result${state.total === 1 ? "" : "s"}`}
          >
            {state.results.map((r) => (
              <CommandItem
                key={r.path}
                value={r.path}
                className="search-result"
                onSelect={() => select(r.path)}
              >
                <div className="search-result__head">
                  <span className="search-result__title">{resultTitle(r)}</span>
                  {r.isMap && (
                    <span className="search-result__meta search-result__meta--map">map</span>
                  )}
                  <span className="search-result__meta">
                    {r.kind ?? "md"}
                    {r.created ? ` · ${r.created}` : ""}
                  </span>
                </div>
                <span className="search-result__path">{r.path}</span>
                {r.snippet && (
                  <span className="search-result__snippet">
                    {highlightSegments(r.snippet, debouncedQuery).map((seg, i) =>
                      seg.match ? (
                        <mark key={i} className="search-result__hit">
                          {seg.text}
                        </mark>
                      ) : (
                        <span key={i}>{seg.text}</span>
                      ),
                    )}
                  </span>
                )}
              </CommandItem>
            ))}
          </CommandGroup>
        )}
      </CommandList>
    </CommandDialog>
  );
}

function PaletteEmpty({
  status,
  error,
  searchable,
}: {
  status: string;
  error: string | null;
  searchable: boolean;
}) {
  if (!searchable) {
    return (
      <CommandEmpty className="search-palette__empty">
        Type at least {MIN_QUERY_LENGTH} characters to search.
      </CommandEmpty>
    );
  }
  if (status === "loading") {
    return (
      <CommandEmpty className="search-palette__empty">
        <SkeletonLines count={3} label="Searching" />
      </CommandEmpty>
    );
  }
  if (status === "error") {
    return <CommandEmpty className="search-palette__empty">Search error: {error}</CommandEmpty>;
  }
  if (status === "ready") {
    return <CommandEmpty className="search-palette__empty">No results.</CommandEmpty>;
  }
  return <CommandEmpty className="search-palette__empty">Search the corpus.</CommandEmpty>;
}
