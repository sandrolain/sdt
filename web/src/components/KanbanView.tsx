/**
 * KanbanView — the `/docs/kanban` surface: the corpus as columns. By default the
 * columns are the declared `status` values; a toggle groups by kind instead. Each
 * column shows at most a chosen number of cards, ordered by a chosen key; the
 * cards are read-only (click opens the document).
 */
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import type { TreeEntry } from "../lib/api";
import { categoryColor, categoryIcon } from "../lib/categories";
import { useCorpusEntries } from "../lib/corpusView";
import { formatFieldDateOnly } from "../lib/frontmatter";
import { Icon } from "../lib/icon";
import { entryKind, kindColor, kindIcon, kindLabel, KIND_ORDER } from "../lib/kinds";
import { displayTitle } from "../lib/titles";
import { SkeletonLines } from "./Skeleton";

type Group = "status" | "kind";
type SortKey = "modified_desc" | "modified_asc" | "title_asc" | "kind_asc";

const SORTS: { id: SortKey; label: string }[] = [
  { id: "modified_desc", label: "Newest" },
  { id: "modified_asc", label: "Oldest" },
  { id: "title_asc", label: "Title A–Z" },
  { id: "kind_asc", label: "Kind" },
];
const LIMITS = [5, 10, 25, 0]; // 0 = all

function sortRows(rows: TreeEntry[], key: SortKey): TreeEntry[] {
  const copy = rows.slice();
  const title = (e: TreeEntry) => displayTitle({ title: e.title, path: e.path });
  const date = (e: TreeEntry) => e.modified ?? e.created ?? "";
  switch (key) {
    case "modified_asc":
      return copy.sort((a, b) => date(a).localeCompare(date(b)));
    case "title_asc":
      return copy.sort((a, b) => title(a).localeCompare(title(b)));
    case "kind_asc":
      return copy.sort((a, b) => entryKind(a).localeCompare(entryKind(b)));
    default:
      return copy.sort((a, b) => date(b).localeCompare(date(a)));
  }
}

export function KanbanView() {
  const { entries, error } = useCorpusEntries();
  const [group, setGroup] = useState<Group>("status");
  const [limit, setLimit] = useState(10);
  const [sortKey, setSortKey] = useState<SortKey>("modified_desc");

  const columns = useMemo(() => {
    const map = new Map<string, TreeEntry[]>();
    for (const entry of entries ?? []) {
      const key = group === "status" ? (entry.status ?? "(no status)") : entryKind(entry);
      const list = map.get(key) ?? [];
      list.push(entry);
      map.set(key, list);
    }
    const keys = [...map.keys()].sort();
    if (group === "kind") {
      keys.sort((a, b) => KIND_ORDER.indexOf(a as never) - KIND_ORDER.indexOf(b as never));
    }
    return keys.map((key) => ({ key, rows: sortRows(map.get(key)!, sortKey) }));
  }, [entries, group, sortKey]);

  if (error) return <p className="content__empty">Kanban error: {error}</p>;
  if (!entries) return <SkeletonLines count={8} label="Loading kanban" />;

  return (
    <div className="kanban">
      <div className="kanban__bar">
        <div className="kanban__modes" role="tablist" aria-label="Group by">
          {(["status", "kind"] as Group[]).map((g) => (
            <button
              key={g}
              type="button"
              role="tab"
              aria-selected={group === g}
              className={`timeline__mode${group === g ? " is-selected" : ""}`}
              onClick={() => setGroup(g)}
            >
              {g === "status" ? "Status" : "Kind"}
            </button>
          ))}
        </div>
        <label className="kanban__control">
          Sort
          <select value={sortKey} onChange={(e) => setSortKey(e.target.value as SortKey)}>
            {SORTS.map((s) => (
              <option key={s.id} value={s.id}>
                {s.label}
              </option>
            ))}
          </select>
        </label>
        <label className="kanban__control">
          Per column
          <select value={limit} onChange={(e) => setLimit(Number(e.target.value))}>
            {LIMITS.map((l) => (
              <option key={l} value={l}>
                {l === 0 ? "All" : l}
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="kanban__board">
        {columns.map((col) => {
          const shown = limit === 0 ? col.rows : col.rows.slice(0, limit);
          const hidden = col.rows.length - shown.length;
          return (
            <section key={col.key} className="kanban__column">
              <h3 className="kanban__column-title">
                {group === "kind" ? kindLabel(col.key as never) : col.key}
                <span className="kanban__column-count">{col.rows.length}</span>
              </h3>
              <ul className="kanban__list" role="list">
                {shown.map((entry) => (
                  <KanbanCard key={entry.path} entry={entry} />
                ))}
              </ul>
              {hidden > 0 && <p className="kanban__more">+{hidden} more</p>}
            </section>
          );
        })}
      </div>
    </div>
  );
}

function KanbanCard({ entry }: { entry: TreeEntry }) {
  const navigate = useNavigate();
  const kind = entryKind(entry);
  const date = entry.modified ?? entry.created;
  return (
    <li>
      <button type="button" className="kanban-card" onClick={() => navigate(`/docs/${entry.path}`)}>
        <span className="kanban-card__head">
          <Icon name={kindIcon(kind)} style={{ color: kindColor(kind) }} title={kindLabel(kind)} />
          <span className="kanban-card__title">
            {displayTitle({ title: entry.title, path: entry.path })}
          </span>
        </span>
        <span className="kanban-card__meta">
          {entry.categories?.[0] && (
            <span style={{ color: categoryColor(entry.categories[0]) }}>
              <Icon name={categoryIcon(entry.categories[0])} /> {entry.categories[0]}
            </span>
          )}
          {date && <span>{formatFieldDateOnly(date)}</span>}
        </span>
      </button>
    </li>
  );
}
