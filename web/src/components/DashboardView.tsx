/**
 * DashboardView — the `/docs/dashboard` surface (the app's default page): the
 * corpus overview as draggable dockview panels (persisted), falling back to a
 * static widget grid in tests. Every widget reads `/api/tree`, except the TODO
 * panel which reads the register through the read-only `/api/todo`.
 */
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { DockviewReact, type IDockviewPanelProps } from "dockview-react";
import type { TodoItem, TodoResponse, TreeEntry } from "../lib/api";
import { categoryColor, categoryIcon } from "../lib/categories";
import { useCorpusEntries } from "../lib/corpusView";
import { fetchJSON } from "../lib/fetchJson";
import { formatFieldDateOnly } from "../lib/frontmatter";
import { Icon } from "../lib/icon";
import { entryKind, kindColor, kindIcon, kindLabel, KIND_ORDER } from "../lib/kinds";
import { displayTitle } from "../lib/titles";
import { addGridPanels, useDockviewTheme, useDockviewWorkspace } from "../lib/dockviewWorkspace";
import { SkeletonLines } from "./Skeleton";

/** A stable "now" for the staleness threshold (never called during render). */
const NOW = Date.now();
const STALE_MS = 90 * 24 * 60 * 60 * 1000;

/** Persisted dockview layout key for the dashboard workspace. */
const DASHBOARD_STORAGE_KEY = "sdt.viewer.dashboardLayout";

function dateOf(entry: TreeEntry): string {
  return entry.modified ?? entry.created ?? "";
}

function computeDashboard(entries: TreeEntry[]) {
  const list = entries;
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
  const openQuestions = list.filter((e) => entryKind(e) === "questions" && e.status !== "resolved");
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
  const kinds = KIND_ORDER.filter((k) => byKind.has(k));
  const topObjectives = [...byObjective.entries()].sort((a, b) => b[1] - a[1]).slice(0, 8);
  const wikiMonths = [...wikiByMonth.entries()].sort((a, b) => a[0].localeCompare(b[0]));
  const taskPct = tasks.length ? Math.round((tasksDone.length / tasks.length) * 100) : 0;
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
    kinds,
    topObjectives,
    wikiMonths,
    taskPct,
    maxCat: Math.max(1, ...byCategory.values()),
    maxObj: Math.max(1, ...byObjective.values()),
    maxWiki: Math.max(1, ...wikiByMonth.values()),
  };
}

type DashboardData = ReturnType<typeof computeDashboard>;

const DashboardDataContext = createContext<DashboardData | null>(null);

function useDashboardData(): DashboardData {
  const data = useContext(DashboardDataContext);
  if (!data) throw new Error("dashboard widget rendered outside its provider");
  return data;
}

function DocumentsWidget() {
  const data = useDashboardData();
  return (
    <>
      <p className="dashboard__big">{data.list.length}</p>
      <ul className="dashboard__stats" role="list">
        {data.kinds.map((kind) => (
          <li key={kind} className="dashboard__stat">
            <Icon name={kindIcon(kind)} style={{ color: kindColor(kind) }} />
            <span className="dashboard__stat-label">{kindLabel(kind)}</span>
            <span className="dashboard__stat-value">{data.byKind.get(kind)}</span>
          </li>
        ))}
      </ul>
    </>
  );
}

function StatusWidget() {
  const data = useDashboardData();
  if (data.byStatus.size === 0) return <p className="content__empty">No statuses.</p>;
  return (
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
  );
}

function RecentListWidget({ entries }: { entries: TreeEntry[] }) {
  if (entries.length === 0) return <p className="content__empty">None.</p>;
  return (
    <ul className="dashboard__recent" role="list">
      {entries.slice(0, 8).map((e) => (
        <DashboardRecent key={e.path} entry={e} />
      ))}
    </ul>
  );
}

function QuestionsWidget() {
  const data = useDashboardData();
  return <RecentListWidget entries={data.openQuestions} />;
}

function PlansWidget() {
  const data = useDashboardData();
  return <RecentListWidget entries={data.activePlans} />;
}

function TasksWidget() {
  const data = useDashboardData();
  return (
    <>
      <p className="dashboard__big">
        {data.taskPct}
        <span className="dashboard__big-unit">%</span>
      </p>
      <p className="dashboard__sub">
        {data.tasksDone.length} / {data.tasks.length} task files completed
      </p>
      <span className="dashboard__bar-track dashboard__bar-track--lg">
        <span className="dashboard__bar-fill" style={{ width: `${data.taskPct}%` }} />
      </span>
    </>
  );
}

function StaleWidget() {
  const data = useDashboardData();
  return (
    <>
      <p className="dashboard__big">{data.stale.length}</p>
      <p className="dashboard__sub">documents not modified in over 90 days</p>
    </>
  );
}

function ObjectivesWidget() {
  const data = useDashboardData();
  if (data.topObjectives.length === 0) return <p className="content__empty">No objectives.</p>;
  return (
    <ul className="dashboard__bars" role="list">
      {data.topObjectives.map(([obj, count]) => (
        <li key={obj} className="dashboard__bar-row">
          <span className="dashboard__bar-label" title={obj}>
            {obj}
          </span>
          <span className="dashboard__bar-track">
            <span
              className="dashboard__bar-fill"
              style={{ width: `${(100 * count) / data.maxObj}%` }}
            />
          </span>
          <span className="dashboard__bar-value">{count}</span>
        </li>
      ))}
    </ul>
  );
}

function WikiWidget() {
  const data = useDashboardData();
  if (data.wikiMonths.length === 0) return <p className="content__empty">No wiki pages.</p>;
  return (
    <div className="dashboard__spark">
      {data.wikiMonths.map(([month, count]) => (
        <span
          key={month}
          className="dashboard__spark-bar"
          style={{ height: `${(100 * count) / data.maxWiki}%` }}
          title={`${month}: ${count}`}
        />
      ))}
    </div>
  );
}

function CategoriesWidget() {
  const data = useDashboardData();
  if (data.byCategory.size === 0) return <p className="content__empty">No categories.</p>;
  return (
    <div className="dashboard__cloud">
      {[...data.byCategory.entries()]
        .sort((a, b) => b[1] - a[1])
        .map(([cat, count]) => (
          <span
            key={cat}
            className="dashboard__tag"
            style={{
              color: categoryColor(cat),
              fontSize: `${0.75 + (0.6 * count) / data.maxCat}rem`,
            }}
          >
            <Icon name={categoryIcon(cat)} />
            {cat} <span className="dashboard__tag-count">{count}</span>
          </span>
        ))}
    </div>
  );
}

function RecentWidget() {
  const data = useDashboardData();
  return (
    <ul className="dashboard__recent dashboard__recent--wide" role="list">
      {data.recent.map((entry) => (
        <DashboardRecent key={entry.path} entry={entry} />
      ))}
    </ul>
  );
}

/** One-shot read of the TODO register; a failure or an absent register is an
 *  empty inbox, never a dashboard error (the register is CLI-owned, optional). */
function useTodos(): { items: TodoItem[]; error: string | null; loading: boolean } {
  const [state, setState] = useState<{ items: TodoItem[]; error: string | null; loading: boolean }>(
    {
      items: [],
      error: null,
      loading: true,
    },
  );
  useEffect(() => {
    let alive = true;
    fetchJSON<TodoResponse>("/api/todo")
      .then((res) => {
        if (alive) setState({ items: res.items ?? [], error: null, loading: false });
      })
      .catch((err: unknown) => {
        if (alive) {
          setState({
            items: [],
            error: err instanceof Error ? err.message : String(err),
            loading: false,
          });
        }
      });
    return () => {
      alive = false;
    };
  }, []);
  return state;
}

/** The TODO inbox, undone first (the promotional half of the register). */
function TodosWidget() {
  const { items, error, loading } = useTodos();
  if (loading) return <SkeletonLines count={4} label="Loading todos" />;
  if (error) return <p className="content__empty">TODO inbox unavailable.</p>;
  if (items.length === 0) return <p className="content__empty">No ideas in the inbox.</p>;
  const ordered = items
    .slice()
    .sort((a, b) => Number(a.done) - Number(b.done) || b.created.localeCompare(a.created));
  return (
    <ul className="dashboard__todos" role="list">
      {ordered.map((it) => (
        <li key={it.id} className={`dashboard__todo${it.done ? " is-done" : ""}`}>
          <Icon name={it.done ? "check_box" : "check_box_outline_blank"} />
          <span className="dashboard__todo-text">{it.text}</span>
        </li>
      ))}
    </ul>
  );
}

/** The dashboard widgets, in order: id, panel title, dockview component, width. */
const WIDGETS: { id: string; title: string; component: string; wide?: boolean }[] = [
  { id: "documents", title: "Documents", component: "documents", wide: true },
  { id: "todos", title: "TODO", component: "todos" },
  { id: "status", title: "Status", component: "status" },
  { id: "questions", title: "Open questions", component: "questions" },
  { id: "plans", title: "Active plans", component: "plans" },
  { id: "tasks", title: "Task progress", component: "tasks" },
  { id: "stale", title: "Stale (90d+)", component: "stale" },
  { id: "objectives", title: "Objectives", component: "objectives" },
  { id: "wiki", title: "Wiki growth", component: "wiki" },
  { id: "categories", title: "Categories", component: "categories" },
  { id: "recent", title: "Recent activity", component: "recent", wide: true },
];

/** Stable dockview component registry (module scope: never recreated). */
const components: Record<string, (props: IDockviewPanelProps) => ReactNode> = Object.fromEntries(
  WIDGETS.map((w) => [
    w.component,
    () => (
      <div className="dock-content">
        <WidgetBody component={w.component} />
      </div>
    ),
  ]),
);

export function DashboardView() {
  const { entries, error } = useCorpusEntries();
  const data = useMemo(() => computeDashboard(entries ?? []), [entries]);
  const dockviewTheme = useDockviewTheme();
  const { enabled, onReady } = useDockviewWorkspace(DASHBOARD_STORAGE_KEY, {
    addPanels: (api) =>
      addGridPanels(
        api,
        WIDGETS.map((w) => ({
          id: w.id,
          component: w.component,
          title: w.title,
          minimumWidth: w.wide ? 300 : 200,
          minimumHeight: 140,
        })),
        3,
      ),
  });

  if (error) return <p className="content__empty">Dashboard error: {error}</p>;
  if (!entries) return <SkeletonLines count={8} label="Loading dashboard" />;

  return (
    <DashboardDataContext.Provider value={data}>
      {enabled ? (
        <DockviewReact
          className="dashboard-workspace dock-layout"
          theme={dockviewTheme}
          components={components}
          onReady={onReady}
        />
      ) : (
        <DashboardFallback />
      )}
    </DashboardDataContext.Provider>
  );
}

/** Test-only stand-in: the same widgets as a static grid (no layout measurement). */
function DashboardFallback() {
  return (
    <div className="dashboard" data-testid="dashboard">
      {WIDGETS.map((w) => (
        <section
          key={w.id}
          className={`dashboard__widget${w.wide ? " dashboard__widget--wide" : ""}`}
        >
          <h3 className="dashboard__title">{w.title}</h3>
          <WidgetBody component={w.component} />
        </section>
      ))}
    </div>
  );
}

function WidgetBody({ component }: { component: string }) {
  switch (component) {
    case "documents":
      return <DocumentsWidget />;
    case "todos":
      return <TodosWidget />;
    case "status":
      return <StatusWidget />;
    case "questions":
      return <QuestionsWidget />;
    case "plans":
      return <PlansWidget />;
    case "tasks":
      return <TasksWidget />;
    case "stale":
      return <StaleWidget />;
    case "objectives":
      return <ObjectivesWidget />;
    case "wiki":
      return <WikiWidget />;
    case "categories":
      return <CategoriesWidget />;
    case "recent":
      return <RecentWidget />;
    default:
      return null;
  }
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
