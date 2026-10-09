/**
 * DashboardView — the `/docs/dashboard` surface (the app's default page): a
 * responsive widget grid over the corpus. Every widget reads `/api/tree`; no
 * extra fetch. A TODO widget is deferred — the todo register has no endpoint.
 */
import { useMemo } from "react";
import { useNavigate } from "react-router-dom";
import type { TreeEntry } from "../lib/api";
import { categoryColor, categoryIcon } from "../lib/categories";
import { useCorpusEntries } from "../lib/corpusView";
import { formatFieldDateOnly } from "../lib/frontmatter";
import { Icon } from "../lib/icon";
import { entryKind, kindColor, kindIcon, kindLabel, KIND_ORDER } from "../lib/kinds";
import { displayTitle } from "../lib/titles";
import { SkeletonLines } from "./Skeleton";

/** A stable "now" for the staleness threshold (never called during render). */
const NOW = Date.now();
const STALE_MS = 90 * 24 * 60 * 60 * 1000;

function dateOf(entry: TreeEntry): string {
  return entry.modified ?? entry.created ?? "";
}

export function DashboardView() {
  const { entries, error } = useCorpusEntries();

  const data = useMemo(() => {
    const list = entries ?? [];
    const byKind = new Map<string, number>();
    const byStatus = new Map<string, number>();
    const byCategory = new Map<string, number>();
    const byObjective = new Map<string, number>();
    const wikiByMonth = new Map<string, number>();
    for (const entry of list) {
      byKind.set(entryKind(entry), (byKind.get(entryKind(entry)) ?? 0) + 1);
      if (entry.status) byStatus.set(entry.status, (byStatus.get(entry.status) ?? 0) + 1);
      for (const cat of entry.categories ?? []) {
        byCategory.set(cat, (byCategory.get(cat) ?? 0) + 1);
      }
      if (entry.objective) {
        byObjective.set(entry.objective, (byObjective.get(entry.objective) ?? 0) + 1);
      }
      if (entryKind(entry) === "wiki" && entry.created) {
        const m = entry.created.slice(0, 7);
        wikiByMonth.set(m, (wikiByMonth.get(m) ?? 0) + 1);
      }
    }
    const openQuestions = list.filter(
      (e) => entryKind(e) === "questions" && e.status !== "resolved",
    );
    const activePlans = list.filter((e) => entryKind(e) === "plan" && e.status === "active");
    const tasks = list.filter((e) => entryKind(e) === "tasks");
    const tasksDone = tasks.filter((e) => e.status === "completed");
    const stale = list.filter((e) => {
      const d = dateOf(e);
      return d ? NOW - Date.parse(d) > STALE_MS : false;
    });
    const recent = list
      .slice()
      .sort((a, b) => dateOf(b).localeCompare(dateOf(a)))
      .slice(0, 12);
    return {
      list,
      byKind,
      byStatus,
      byCategory,
      byObjective,
      wikiByMonth,
      openQuestions,
      activePlans,
      tasks,
      tasksDone,
      stale,
      recent,
    };
  }, [entries]);

  if (error) return <p className="content__empty">Dashboard error: {error}</p>;
  if (!entries) return <SkeletonLines count={8} label="Loading dashboard" />;

  const kinds = KIND_ORDER.filter((k) => data.byKind.has(k));
  const maxCat = Math.max(1, ...data.byCategory.values());
  const maxObj = Math.max(1, ...data.byObjective.values());
  const topObjectives = [...data.byObjective.entries()].sort((a, b) => b[1] - a[1]).slice(0, 8);
  const wikiMonths = [...data.wikiByMonth.entries()].sort((a, b) => a[0].localeCompare(b[0]));
  const maxWiki = Math.max(1, ...data.wikiByMonth.values());
  const taskPct = data.tasks.length
    ? Math.round((data.tasksDone.length / data.tasks.length) * 100)
    : 0;

  return (
    <div className="dashboard">
      <section className="dashboard__widget dashboard__widget--wide">
        <h3 className="dashboard__title">Documents</h3>
        <p className="dashboard__big">{data.list.length}</p>
        <ul className="dashboard__stats" role="list">
          {kinds.map((kind) => (
            <li key={kind} className="dashboard__stat">
              <Icon name={kindIcon(kind)} style={{ color: kindColor(kind) }} />
              <span className="dashboard__stat-label">{kindLabel(kind)}</span>
              <span className="dashboard__stat-value">{data.byKind.get(kind)}</span>
            </li>
          ))}
        </ul>
      </section>

      <section className="dashboard__widget">
        <h3 className="dashboard__title">Status</h3>
        {data.byStatus.size === 0 ? (
          <p className="content__empty">No statuses.</p>
        ) : (
          <ul className="dashboard__bars" role="list">
            {[...data.byStatus.entries()]
              .sort((a, b) => b[1] - a[1])
              .map(([status, count]) => (
                <li key={status} className="dashboard__bar-row">
                  <span className="dashboard__bar-label">{status}</span>
                  <span className="dashboard__bar-track">
                    <span
                      className="dashboard__bar-fill"
                      style={{ width: `${(100 * count) / data.list.length}%` }}
                    />
                  </span>
                  <span className="dashboard__bar-value">{count}</span>
                </li>
              ))}
          </ul>
        )}
      </section>

      <section className="dashboard__widget">
        <h3 className="dashboard__title">Open questions</h3>
        {data.openQuestions.length === 0 ? (
          <p className="content__empty">None open.</p>
        ) : (
          <ul className="dashboard__recent" role="list">
            {data.openQuestions.slice(0, 8).map((e) => (
              <DashboardRecent key={e.path} entry={e} />
            ))}
          </ul>
        )}
      </section>

      <section className="dashboard__widget">
        <h3 className="dashboard__title">Active plans</h3>
        {data.activePlans.length === 0 ? (
          <p className="content__empty">None active.</p>
        ) : (
          <ul className="dashboard__recent" role="list">
            {data.activePlans.slice(0, 8).map((e) => (
              <DashboardRecent key={e.path} entry={e} />
            ))}
          </ul>
        )}
      </section>

      <section className="dashboard__widget">
        <h3 className="dashboard__title">Task progress</h3>
        <p className="dashboard__big">
          {taskPct}
          <span className="dashboard__big-unit">%</span>
        </p>
        <p className="dashboard__sub">
          {data.tasksDone.length} / {data.tasks.length} task files completed
        </p>
        <span className="dashboard__bar-track dashboard__bar-track--lg">
          <span className="dashboard__bar-fill" style={{ width: `${taskPct}%` }} />
        </span>
      </section>

      <section className="dashboard__widget">
        <h3 className="dashboard__title">Stale (90d+)</h3>
        <p className="dashboard__big">{data.stale.length}</p>
        <p className="dashboard__sub">documents not modified in over 90 days</p>
      </section>

      <section className="dashboard__widget">
        <h3 className="dashboard__title">Objectives</h3>
        {topObjectives.length === 0 ? (
          <p className="content__empty">No objectives.</p>
        ) : (
          <ul className="dashboard__bars" role="list">
            {topObjectives.map(([obj, count]) => (
              <li key={obj} className="dashboard__bar-row">
                <span className="dashboard__bar-label" title={obj}>
                  {obj}
                </span>
                <span className="dashboard__bar-track">
                  <span
                    className="dashboard__bar-fill"
                    style={{ width: `${(100 * count) / maxObj}%` }}
                  />
                </span>
                <span className="dashboard__bar-value">{count}</span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="dashboard__widget">
        <h3 className="dashboard__title">Wiki growth</h3>
        {wikiMonths.length === 0 ? (
          <p className="content__empty">No wiki pages.</p>
        ) : (
          <div className="dashboard__spark">
            {wikiMonths.map(([month, count]) => (
              <span
                key={month}
                className="dashboard__spark-bar"
                style={{ height: `${(100 * count) / maxWiki}%` }}
                title={`${month}: ${count}`}
              />
            ))}
          </div>
        )}
      </section>

      <section className="dashboard__widget">
        <h3 className="dashboard__title">Categories</h3>
        {data.byCategory.size === 0 ? (
          <p className="content__empty">No categories.</p>
        ) : (
          <div className="dashboard__cloud">
            {[...data.byCategory.entries()]
              .sort((a, b) => b[1] - a[1])
              .map(([cat, count]) => (
                <span
                  key={cat}
                  className="dashboard__tag"
                  style={{
                    color: categoryColor(cat),
                    fontSize: `${0.75 + (0.6 * count) / maxCat}rem`,
                  }}
                >
                  <Icon name={categoryIcon(cat)} />
                  {cat} <span className="dashboard__tag-count">{count}</span>
                </span>
              ))}
          </div>
        )}
      </section>

      <section className="dashboard__widget dashboard__widget--wide">
        <h3 className="dashboard__title">Recent activity</h3>
        <ul className="dashboard__recent dashboard__recent--wide" role="list">
          {data.recent.map((entry) => (
            <DashboardRecent key={entry.path} entry={entry} />
          ))}
        </ul>
      </section>
    </div>
  );
}

function DashboardRecent({ entry }: { entry: TreeEntry }) {
  const navigate = useNavigate();
  const kind = entryKind(entry);
  const date = dateOf(entry);
  return (
    <li>
      <button
        type="button"
        className="dashboard__recent-row"
        onClick={() => navigate(`/docs/${entry.path}`)}
      >
        <Icon name={kindIcon(kind)} style={{ color: kindColor(kind) }} title={kindLabel(kind)} />
        <span className="dashboard__recent-title">
          {displayTitle({ title: entry.title, path: entry.path })}
        </span>
        {date && <span className="dashboard__recent-date">{formatFieldDateOnly(date)}</span>}
      </button>
    </li>
  );
}
