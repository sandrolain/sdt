import { useEffect, useMemo, useState } from "react";
import { NavLink } from "react-router-dom";
import { fetchTree, type TreeEntry } from "../lib/api";
import { entryKind, kindLabel, availableKinds } from "../lib/kinds";

export function Tree() {
  const [entries, setEntries] = useState<TreeEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [kind, setKind] = useState<string>("all");

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
  }, []);

  const kinds = useMemo(() => (entries ? availableKinds(entries) : []), [entries]);

  const filtered = useMemo(
    () =>
      (entries ?? []).filter((e) => {
        if (kind === "all") return true;
        return entryKind(e) === kind;
      }),
    [entries, kind],
  );

  return (
    <aside className="panel panel--tree">
      <div className="panel-header">
        <span className="panel-header__title">Tree</span>
        <select
          className="kind-filter"
          aria-label="Filter by kind"
          value={kind}
          onChange={(e) => setKind(e.target.value)}
        >
          <option value="all">all</option>
          {kinds.map((k) => (
            <option key={k} value={k}>
              {kindLabel(k)}
            </option>
          ))}
        </select>
      </div>
      {error ? (
        <p className="content__empty">Tree error: {error}</p>
      ) : !entries ? (
        <p className="content__empty">Loading tree…</p>
      ) : (
        <ul role="list">
          {filtered.map((e) => (
            <li key={e.path}>
              <NavLink
                to={`/docs/${e.path}`}
                className="tree-entry"
                title={e.summary || e.path}
                end
              >
                <span className="tree-entry__title">{entryTitle(e)}</span>
                {e.isMap && <span className="tree-entry__kind tree-entry__kind--map">map</span>}
                <span className={`tree-entry__kind${e.canvas ? " tree-entry__kind--canvas" : ""}`}>
                  {e.canvas ? "canvas" : (e.kind ?? "md")}
                </span>
              </NavLink>
            </li>
          ))}
        </ul>
      )}
    </aside>
  );
}

function entryTitle(e: TreeEntry): string {
  if (e.title) return e.title;
  if (e.canvas) return e.path;
  const base = e.path.split("/").pop() ?? e.path;
  return base.endsWith(".md") ? base.slice(0, -3) : base;
}
