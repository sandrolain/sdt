import { useEffect, useMemo, useState } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { fetchTree, type TreeEntry } from "../lib/api";
import { entryKind, kindColor, kindIcon, kindLabel, type EntryFilterKind } from "../lib/kinds";
import { MAP_ICON } from "../lib/documentModes";
import { imageUrl } from "../lib/images";
import { Icon } from "../lib/icon";
import { SkeletonLines } from "./Skeleton";
import { displayTitle, filenameDate } from "../lib/titles";
import { formatFieldDate } from "../lib/frontmatter";
import {
  groupByKind,
  sortEntries,
  TREE_SORTS,
  type TreeDir,
  type TreeSortKey,
} from "../lib/treeSort";
import { planReferencedAnalyses, statusDot } from "../lib/statusDot";
import { useReloadToken } from "../lib/useReloadToken";
import { Select } from "./ui/Select";

export function Tree() {
  const [entries, setEntries] = useState<TreeEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<TreeSortKey>("created");
  const [dir, setDir] = useState<TreeDir>("desc");
  const [openKinds, setOpenKinds] = useState<Set<EntryFilterKind>>(() => new Set());
  const reloadToken = useReloadToken();
  const location = useLocation();

  useEffect(() => {
    let alive = true;
    fetchTree()
      .then((res) => {
        if (alive) setEntries(res.entries);
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      alive = false;
    };
  }, [reloadToken]);

  // kind section of the document currently open in the documents route
  const activeKind = useMemo(() => {
    if (!entries || !location.pathname.startsWith("/docs/")) return null;
    const path = decodeURIComponent(location.pathname.slice("/docs/".length));
    const entry = entries.find((e) => e.path === path);
    return entry ? entryKind(entry) : null;
  }, [entries, location.pathname]);

  // reveal the active entry once its section is open (sections are derived-open)
  useEffect(() => {
    if (!entries || !activeKind) return;
    document
      .querySelector(".panel--tree .tree-entry.is-active")
      ?.scrollIntoView({ block: "nearest" });
  }, [entries, location.pathname, activeKind]);

  const groups = entries
    ? groupByKind(entries).map((group) => ({
        kind: group.kind,
        entries: sortEntries(group.entries, sortKey, dir),
      }))
    : [];
  const plannedAnalyses = planReferencedAnalyses(entries ?? []);

  return (
    <aside className="panel panel--tree" aria-label="Corpus tree">
      <div className="panel-header">
        <span className="panel-header__title">Tree</span>
        <Select
          ariaLabel="Sort entries by"
          className="ui-select--compact"
          options={TREE_SORTS.map((s) => ({ id: s.id, label: s.label }))}
          selectedKey={sortKey}
          onSelectionChange={(key) => setSortKey(String(key) as TreeSortKey)}
        />
        <button
          type="button"
          className="tree-sort-dir"
          aria-label={dir === "asc" ? "Sort ascending" : "Sort descending"}
          title={dir === "asc" ? "Ascending" : "Descending"}
          onClick={() => setDir((d) => (d === "asc" ? "desc" : "asc"))}
        >
          <Icon name={dir === "asc" ? "arrow_upward" : "arrow_downward"} />
        </button>
      </div>
      {error ? (
        <p className="content__empty">Tree error: {error}</p>
      ) : !entries ? (
        <SkeletonLines count={6} label="Loading tree" />
      ) : (
        <div className="tree-groups">
          {groups.map((group) => (
            <details
              key={group.kind}
              className="tree-folder"
              open={openKinds.has(group.kind) || activeKind === group.kind}
              onToggle={(e) => {
                const open = e.currentTarget.open;
                setOpenKinds((prev) => {
                  const next = new Set(prev);
                  if (open) next.add(group.kind);
                  else next.delete(group.kind);
                  return next;
                });
              }}
            >
              <summary className="tree-folder__header">
                <Icon name="expand_more" className="tree-folder__chevron" />
                <Icon
                  name={kindIcon(group.kind)}
                  className="tree-folder__icon"
                  style={{ color: kindColor(group.kind) }}
                />
                <span className="tree-folder__label">{kindLabel(group.kind)}</span>
                <span className="tree-folder__count">{group.entries.length}</span>
              </summary>
              <ul role="list">
                {group.entries.map((entry) => {
                  const dot = statusDot(entry, plannedAnalyses);
                  return (
                    <li key={entry.path}>
                      <NavLink
                        to={
                          entry.canvas
                            ? `/wiki/board?file=${encodeURIComponent(entry.path)}`
                            : `/docs/${entry.path}`
                        }
                        className="tree-entry"
                        title={entry.summary || entry.path}
                        end
                      >
                        <span className="tree-entry__glyph">
                          {entry.image ? (
                            <img
                              className="tree-entry__thumb"
                              src={imageUrl(entry.image, entry.path)}
                              alt=""
                            />
                          ) : (
                            <Icon name={kindIcon(entryKind(entry))} />
                          )}
                        </span>
                        <span className="tree-entry__text">
                          <span className="tree-entry__title">{entryTitle(entry)}</span>
                          {entryDate(entry) && (
                            <span className="tree-entry__date">{entryDate(entry)}</span>
                          )}
                        </span>
                        {dot && (
                          <span
                            className={`tree-entry__dot tree-entry__dot--${dot.tone}`}
                            title={dot.label}
                            aria-label={dot.label}
                            role="img"
                          />
                        )}
                        {entry.isMap && (
                          <span className="tree-entry__map" title="Map document">
                            <Icon name={MAP_ICON} label="Map document" />
                          </span>
                        )}
                        <span
                          className={`tree-entry__kind${entry.canvas ? " tree-entry__kind--canvas" : ""}`}
                        >
                          {entry.canvas ? "canvas" : (entry.kind ?? "md")}
                        </span>
                      </NavLink>
                    </li>
                  );
                })}
              </ul>
            </details>
          ))}
        </div>
      )}
    </aside>
  );
}

function entryTitle(e: TreeEntry): string {
  if (e.canvas) return e.path;
  return displayTitle({ title: e.title, path: e.path });
}

/** Small date line: frontmatter `created`, else the filename date prefix. */
function entryDate(e: TreeEntry): string {
  const raw = e.created || filenameDate(e.path);
  return raw ? formatFieldDate(raw) : "";
}
