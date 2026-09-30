import { useEffect, useReducer, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "cmdk";
import { fetchSearch, fetchVocab, type VocabResponse } from "../lib/api";
import { MAP_ICON } from "../lib/documentModes";
import { formatFieldDateOnly } from "../lib/frontmatter";
import { kindLabel } from "../lib/kinds";
import { Icon } from "../lib/icon";
import { STATUS_VALUES } from "../lib/frontmatter";
import { SkeletonLines } from "./Skeleton";
import {
  hasMore,
  KIND_OPTIONS,
  paletteReducer,
  initialPaletteState,
  highlightSegments,
  resultRoute,
  resultTitle,
  useDebouncedValue,
} from "../lib/searchPalette";

interface SearchPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * The status vocabulary of the matrix, flattened for one select. Keys are
 * `<kind>.<value>`; the first kind that offers a value supplies its label, so a
 * shared value reads once rather than once per kind.
 */
function statusOptions(): { id: string; label: string }[] {
  const seen = new Map<string, string>();
  for (const [key, entry] of Object.entries(STATUS_VALUES)) {
    const value = key.slice(key.indexOf(".") + 1);
    if (!seen.has(value)) seen.set(value, entry.label);
  }
  return [...seen.entries()]
    .map(([id, label]) => ({ id, label }))
    .sort((a, b) => a.id.localeCompare(b.id));
}

/**
 * Modal command palette over /api/search. With no query it browses the corpus
 * (server order `modified_desc`); with one it ranks by relevance. The filters
 * come from the corpus registers (`/api/vocab`) plus the status matrix, so the
 * palette offers values the corpus accepts instead of free text.
 */
export function SearchPalette({ open, onOpenChange }: SearchPaletteProps) {
  const [state, dispatch] = useReducer(paletteReducer, initialPaletteState);
  const [vocab, setVocab] = useState<VocabResponse | null>(null);
  const debouncedQuery = useDebouncedValue(state.query, 200);
  const navigate = useNavigate();
  const { kind, objective, status, topic, category, from, to } = state.filters;

  useEffect(() => {
    if (!open) {
      dispatch({ type: "resetFilters" });
      dispatch({ type: "cleared" });
    }
  }, [open]);

  // the registers change rarely: fetch once per session, cache in the module
  useEffect(() => {
    if (vocab) return;
    let alive = true;
    fetchVocab()
      .then((res) => {
        if (alive) setVocab(res);
      })
      .catch(() => {
        // free-form facets degrade to the empty option
      });
    return () => {
      alive = false;
    };
  }, [vocab]);

  useEffect(() => {
    let alive = true;
    dispatch({ type: "load" });
    fetchSearch({
      q: debouncedQuery.trim(),
      kind,
      objective,
      status,
      topic,
      category,
      from,
      to,
      limit: state.limit,
    })
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
  }, [debouncedQuery, kind, objective, status, topic, category, from, to, state.limit]);

  const select = (path: string, section?: string) => {
    onOpenChange(false);
    navigate(resultRoute(path, section));
  };

  const more = hasMore(state);

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
        role="group"
        aria-label="Search filters"
        tabIndex={0}
        onKeyDown={(e) => {
          // Arrow keys belong to the control that received them (a select
          // changes value); only the row itself lets them through, so focus
          // parked here still arrows into the results instead of dead-ending.
          if (e.target !== e.currentTarget && e.key.startsWith("Arrow")) e.stopPropagation();
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
                {kindLabel(k)}
              </option>
            ))}
          </select>
        </label>
        <FacetSelect
          label="Status"
          value={status}
          options={statusOptions()}
          onChange={(value) => dispatch({ type: "status", value })}
        />
        {(vocab?.objectives ?? []).length > 0 && (
          <FacetSelect
            label="Objective"
            value={objective}
            options={(vocab?.objectives ?? []).map((id) => ({ id, label: id }))}
            onChange={(value) => dispatch({ type: "objective", value })}
          />
        )}
        {(vocab?.topics ?? []).length > 0 && (
          <FacetSelect
            label="Topic"
            value={topic}
            options={(vocab?.topics ?? []).map((id) => ({ id, label: id }))}
            onChange={(value) => dispatch({ type: "topic", value })}
          />
        )}
        {(vocab?.categories ?? []).length > 0 && (
          <FacetSelect
            label="Category"
            value={category}
            options={(vocab?.categories ?? []).map((id) => ({ id, label: id }))}
            onChange={(value) => dispatch({ type: "category", value })}
          />
        )}
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
        <PaletteEmpty status={state.status} error={state.error} />
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
                onSelect={() => select(r.path, r.section)}
              >
                <div className="search-result__head">
                  <span className="search-result__title">{resultTitle(r)}</span>
                  {r.isMap && <Icon name={MAP_ICON} className="map-icon" label="Map document" />}
                  <span className="search-result__meta">
                    {r.kind ?? "md"}
                    {r.created ? ` · ${formatFieldDateOnly(r.created)}` : ""}
                    {r.modified && r.modified !== r.created
                      ? ` · updated ${formatFieldDateOnly(r.modified)}`
                      : ""}
                    {r.section ? " · §" : ""}
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
      <div className="search-palette__footer">
        {/* the count is the async state a screen reader needs announced */}
        <span className="search-palette__status" role="status">
          {state.status === "loading"
            ? "Searching…"
            : state.status === "error"
              ? "Search failed"
              : `${state.results.length} of ${state.total} shown`}
        </span>
        {more && (
          <button
            type="button"
            className="search-palette__more"
            onClick={() => dispatch({ type: "more" })}
          >
            Show more
          </button>
        )}
      </div>
    </CommandDialog>
  );
}

/** One register-backed facet: a select over a controlled vocabulary. */
function FacetSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { id: string; label: string }[];
  onChange: (value: string) => void;
}) {
  return (
    <label className="search-filters__field">
      <span className="search-filters__label">{label}</span>
      <select
        className="search-filters__control"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        <option value="">any</option>
        {options.map((o) => (
          <option key={o.id} value={o.id}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}

function PaletteEmpty({ status, error }: { status: string; error: string | null }) {
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
