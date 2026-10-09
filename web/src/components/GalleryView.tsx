/**
 * GalleryView — the `/docs/gallery` surface: the corpus as a card grid, one card
 * per document (thumbnail, kind, category, dates, status, summary), with a kind
 * filter and a newest-first order. Clicking a card opens the document.
 */
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import type { TreeEntry } from "../lib/api";
import { categoryColor, categoryIcon } from "../lib/categories";
import { useCorpusEntries } from "../lib/corpusView";
import { formatFieldDateOnly } from "../lib/frontmatter";
import { Icon } from "../lib/icon";
import { imageUrl } from "../lib/images";
import { entryKind, kindColor, kindIcon, kindLabel, KIND_ORDER } from "../lib/kinds";
import { entryState } from "../lib/statusDot";
import { displayTitle } from "../lib/titles";
import { SkeletonLines } from "./Skeleton";

const ALL = "all";
const PAGE_SIZES = [12, 24, 48, 96];
const PAGE_SIZE_KEY = "sdt-gallery-page-size";

function loadPageSize(): number {
  try {
    const raw = Number(localStorage.getItem(PAGE_SIZE_KEY));
    return PAGE_SIZES.includes(raw) ? raw : 24;
  } catch {
    return 24;
  }
}

export function GalleryView() {
  const { entries, error } = useCorpusEntries();
  const [kindFilter, setKindFilter] = useState<string>(ALL);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState<number>(loadPageSize);
  const navigate = useNavigate();

  const rows = useMemo(() => {
    if (!entries) return [];
    const filtered =
      kindFilter === ALL ? entries : entries.filter((e) => entryKind(e) === kindFilter);
    return filtered
      .slice()
      .sort((a, b) => (b.modified ?? b.created ?? "").localeCompare(a.modified ?? a.created ?? ""));
  }, [entries, kindFilter]);

  const pageCount = Math.max(1, Math.ceil(rows.length / pageSize));
  const safePage = Math.min(page, pageCount - 1);
  const pageRows = rows.slice(safePage * pageSize, safePage * pageSize + pageSize);

  if (error) return <p className="content__empty">Gallery error: {error}</p>;
  if (!entries) return <SkeletonLines count={8} label="Loading gallery" />;

  const chooseSize = (size: number) => {
    setPageSize(size);
    setPage(0);
    try {
      localStorage.setItem(PAGE_SIZE_KEY, String(size));
    } catch {
      // persistence is non-essential
    }
  };

  return (
    <div className="gallery">
      <div className="gallery__bar">
        <div className="gallery__filter" role="group" aria-label="Filter by kind">
          <button
            type="button"
            className={`gallery__chip${kindFilter === ALL ? " is-active" : ""}`}
            onClick={() => {
              setKindFilter(ALL);
              setPage(0);
            }}
          >
            All
          </button>
          {KIND_ORDER.map((kind) => (
            <button
              key={kind}
              type="button"
              className={`gallery__chip${kindFilter === kind ? " is-active" : ""}`}
              onClick={() => {
                setKindFilter(kind);
                setPage(0);
              }}
            >
              <Icon name={kindIcon(kind)} style={{ color: kindColor(kind) }} />
              {kindLabel(kind)}
            </button>
          ))}
        </div>
        <span className="gallery__count">{rows.length}</span>
      </div>
      <div className="gallery__grid">
        {pageRows.map((entry) => (
          <GalleryCard
            key={entry.path}
            entry={entry}
            onOpen={() => navigate(`/docs/${entry.path}`)}
          />
        ))}
      </div>
      <div className="gallery__pager">
        <label className="gallery__pagesize">
          Per page
          <select value={pageSize} onChange={(e) => chooseSize(Number(e.target.value))}>
            {PAGE_SIZES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </label>
        <div className="gallery__nav">
          <button
            type="button"
            className="gallery__nav-btn"
            disabled={safePage === 0}
            onClick={() => setPage(safePage - 1)}
            aria-label="Previous page"
          >
            <Icon name="chevron_left" />
          </button>
          <span className="gallery__page-label">
            {safePage + 1} / {pageCount}
          </span>
          <button
            type="button"
            className="gallery__nav-btn"
            disabled={safePage >= pageCount - 1}
            onClick={() => setPage(safePage + 1)}
            aria-label="Next page"
          >
            <Icon name="chevron_right" />
          </button>
        </div>
      </div>
    </div>
  );
}

function GalleryCard({ entry, onOpen }: { entry: TreeEntry; onOpen: () => void }) {
  const kind = entryKind(entry);
  const state = entryState(entry, new Map(), new Map());
  const date = entry.modified ?? entry.created;
  return (
    <button type="button" className="gallery-card" onClick={onOpen}>
      {entry.image && (
        <img className="gallery-card__thumb" src={imageUrl(entry.image, entry.path)} alt="" />
      )}
      <span className="gallery-card__head">
        <Icon name={kindIcon(kind)} style={{ color: kindColor(kind) }} title={kindLabel(kind)} />
        <span className="gallery-card__title">
          {displayTitle({ title: entry.title, path: entry.path })}
        </span>
        {state.key !== "no-state" && (
          <span
            className={`gallery-card__dot tree-entry__dot--${state.tone}`}
            title={state.label}
            aria-label={state.label}
            role="img"
          />
        )}
      </span>
      {entry.summary && <span className="gallery-card__summary">{entry.summary}</span>}
      <span className="gallery-card__meta">
        {entry.categories?.[0] && (
          <span
            className="gallery-card__category"
            style={{ color: categoryColor(entry.categories[0]) }}
          >
            <Icon name={categoryIcon(entry.categories[0])} />
            {entry.categories[0]}
          </span>
        )}
        {date && <span className="gallery-card__date">{formatFieldDateOnly(date)}</span>}
      </span>
    </button>
  );
}
