/**
 * TimelineView — the `/docs/timeline` surface: the corpus over time. Calendar,
 * heatmap and the month's document list are draggable dockview panels
 * (persisted); the list carries a kind filter and pagination. Falls back to a
 * static two-column layout in tests.
 */
import { createContext, useContext, useMemo, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { DockviewReact, type IDockviewPanelProps } from "dockview-react";
import type { TreeEntry } from "../lib/api";
import { useCorpusEntries } from "../lib/corpusView";
import { formatFieldDateOnly } from "../lib/frontmatter";
import { Icon } from "../lib/icon";
import { availableKinds, entryKind, kindColor, kindIcon, kindLabel } from "../lib/kinds";
import { displayTitle } from "../lib/titles";
import { useDockviewTheme, useDockviewWorkspace } from "../lib/dockviewWorkspace";
import { SkeletonLines } from "./Skeleton";

/** A stable "today" for the empty-corpus fallback (never called during render). */
const TODAY = new Date().toISOString().slice(0, 10);

/** Days shown per page in the list panel. */
const LIST_PAGE_SIZE = 10;

/** Persisted dockview layout key for the timeline workspace. */
const TIMELINE_STORAGE_KEY = "sdt.viewer.timelineLayout";

/** The ISO day (YYYY-MM-DD) a document is dated on, or null. */
function entryDay(entry: TreeEntry): string | null {
  const raw = entry.modified ?? entry.created;
  return raw ? raw.slice(0, 10) : null;
}

interface TimelineData {
  byDay: Map<string, TreeEntry[]>;
  entries: TreeEntry[];
  month: string;
  setMonth: (m: string) => void;
}

const TimelineDataContext = createContext<TimelineData | null>(null);

function useTimelineData(): TimelineData {
  const data = useContext(TimelineDataContext);
  if (!data) throw new Error("timeline panel rendered outside its provider");
  return data;
}

export function TimelineView() {
  const { entries, error } = useCorpusEntries();
  const [month, setMonth] = useState<string>("");
  const dockviewTheme = useDockviewTheme();
  const { enabled, onReady } = useDockviewWorkspace(TIMELINE_STORAGE_KEY, {
    addPanels: (api) => {
      if (!api.getPanel("calendar")) {
        api.addPanel({
          id: "calendar",
          component: "calendar",
          title: "Calendar",
          minimumWidth: 240,
          minimumHeight: 220,
        });
      }
      if (!api.getPanel("heatmap")) {
        api.addPanel({
          id: "heatmap",
          component: "heatmap",
          title: "Heatmap",
          minimumWidth: 240,
          minimumHeight: 120,
          position: { referencePanel: "calendar", direction: "below" },
        });
      }
      if (!api.getPanel("list")) {
        api.addPanel({
          id: "list",
          component: "list",
          title: "Documents",
          minimumWidth: 320,
          minimumHeight: 220,
          position: { referencePanel: "calendar", direction: "right" },
        });
      }
    },
  });

  const byDay = useMemo(() => {
    const map = new Map<string, TreeEntry[]>();
    for (const entry of entries ?? []) {
      const day = entryDay(entry);
      if (!day) continue;
      const list = map.get(day) ?? [];
      list.push(entry);
      map.set(day, list);
    }
    return map;
  }, [entries]);

  // The selected month defaults to the most recent document's month.
  const latestMonth = useMemo(() => {
    const days = [...byDay.keys()].sort((a, b) => b.localeCompare(a));
    return (days[0] ?? TODAY).slice(0, 7);
  }, [byDay]);
  const selectedMonth = month || latestMonth;

  if (error) return <p className="content__empty">Timeline error: {error}</p>;
  if (!entries) return <SkeletonLines count={8} label="Loading timeline" />;

  const data: TimelineData = { byDay, entries, month: selectedMonth, setMonth };

  return (
    <TimelineDataContext.Provider value={data}>
      {enabled ? (
        <DockviewReact
          className="timeline-workspace dock-layout"
          theme={dockviewTheme}
          components={timelineComponents}
          onReady={onReady}
        />
      ) : (
        <TimelineFallback />
      )}
    </TimelineDataContext.Provider>
  );
}

/** Stable dockview component registry (module scope: never recreated). */
const timelineComponents: Record<string, (props: IDockviewPanelProps) => ReactNode> = {
  calendar: () => (
    <div className="dock-content">
      <CalendarPanel />
    </div>
  ),
  heatmap: () => (
    <div className="dock-content">
      <HeatmapPanel />
    </div>
  ),
  list: () => (
    <div className="dock-content">
      <ListPanel />
    </div>
  ),
};

function CalendarPanel() {
  const { byDay, month, setMonth } = useTimelineData();
  return <Calendar byDay={byDay} month={month} onMonth={setMonth} />;
}

function HeatmapPanel() {
  const { byDay } = useTimelineData();
  return <Heatmap byDay={byDay} />;
}

function ListPanel() {
  const { byDay, entries, month } = useTimelineData();
  const [kindFilter, setKindFilter] = useState<string>("all");
  const [page, setPage] = useState(0);

  const kinds = useMemo(() => availableKinds(entries), [entries]);
  const days = useMemo(
    () =>
      [...byDay.keys()]
        .filter((d) => d.startsWith(month))
        .sort((a, b) => b.localeCompare(a))
        .map((day) => ({
          day,
          docs: (byDay.get(day) ?? []).filter(
            (e) => kindFilter === "all" || entryKind(e) === kindFilter,
          ),
        }))
        .filter((d) => d.docs.length > 0),
    [byDay, month, kindFilter],
  );
  const pages = Math.max(1, Math.ceil(days.length / LIST_PAGE_SIZE));
  const current = Math.min(page, pages - 1);
  const slice = days.slice(current * LIST_PAGE_SIZE, current * LIST_PAGE_SIZE + LIST_PAGE_SIZE);

  return (
    <div className="timeline__panel">
      <div className="timeline__controls">
        <label className="timeline__filter">
          <span className="timeline__filter-label">Kind</span>
          <select
            value={kindFilter}
            onChange={(e) => {
              setKindFilter(e.target.value);
              setPage(0);
            }}
          >
            <option value="all">All</option>
            {kinds.map((k) => (
              <option key={k} value={k}>
                {kindLabel(k)}
              </option>
            ))}
          </select>
        </label>
        {pages > 1 && (
          <div className="timeline__pager">
            <button
              type="button"
              className="timeline__pager-btn"
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={current === 0}
              aria-label="Previous page"
            >
              <Icon name="chevron_left" />
            </button>
            <span className="timeline__pager-label">
              {current + 1} / {pages}
            </span>
            <button
              type="button"
              className="timeline__pager-btn"
              onClick={() => setPage((p) => Math.min(pages - 1, p + 1))}
              disabled={current >= pages - 1}
              aria-label="Next page"
            >
              <Icon name="chevron_right" />
            </button>
          </div>
        )}
      </div>
      <MonthTimeline days={slice} />
    </div>
  );
}

function MonthTimeline({ days }: { days: { day: string; docs: TreeEntry[] }[] }) {
  if (days.length === 0) {
    return <p className="content__empty">No documents.</p>;
  }
  return (
    <div className="timeline__list">
      {days.map(({ day, docs }) => (
        <section key={day} className="timeline__day">
          <h3 className="timeline__date">{formatFieldDateOnly(day)}</h3>
          <ul className="timeline__docs" role="list">
            {docs.map((entry) => (
              <TimelineRow key={entry.path} entry={entry} />
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}

function TimelineRow({ entry }: { entry: TreeEntry }) {
  const navigate = useNavigate();
  const kind = entryKind(entry);
  return (
    <li>
      <button
        type="button"
        className="timeline__doc"
        onClick={() => navigate(`/docs/${entry.path}`)}
      >
        <Icon name={kindIcon(kind)} style={{ color: kindColor(kind) }} title={kindLabel(kind)} />
        <span className="timeline__doc-title">
          {displayTitle({ title: entry.title, path: entry.path })}
        </span>
      </button>
    </li>
  );
}

function Calendar({
  byDay,
  month,
  onMonth,
}: {
  byDay: Map<string, TreeEntry[]>;
  month: string;
  onMonth: (m: string) => void;
}) {
  const [year, mon] = (month || TODAY.slice(0, 7)).split("-").map(Number);
  const startPad = (new Date(Date.UTC(year, mon - 1, 1)).getUTCDay() + 6) % 7; // Monday-first
  const daysInMonth = new Date(Date.UTC(year, mon, 0)).getUTCDate();
  const shift = (delta: number) =>
    onMonth(new Date(Date.UTC(year, mon - 1 + delta, 1)).toISOString().slice(0, 7));
  return (
    <div className="calendar">
      <div className="calendar__head">
        <button
          type="button"
          className="calendar__nav"
          onClick={() => shift(-1)}
          aria-label="Previous month"
        >
          <Icon name="chevron_left" />
        </button>
        <span className="calendar__label">{month || TODAY.slice(0, 7)}</span>
        <button
          type="button"
          className="calendar__nav"
          onClick={() => shift(1)}
          aria-label="Next month"
        >
          <Icon name="chevron_right" />
        </button>
      </div>
      <div className="calendar__grid">
        {["Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"].map((d) => (
          <span key={d} className="calendar__dow">
            {d}
          </span>
        ))}
        {Array.from({ length: startPad }).map((_, i) => (
          <span key={`pad-${i}`} className="calendar__cell calendar__cell--pad" />
        ))}
        {Array.from({ length: daysInMonth }).map((_, i) => {
          const day = `${month}-${String(i + 1).padStart(2, "0")}`;
          const count = byDay.get(day)?.length ?? 0;
          return (
            <span
              key={day}
              className={`calendar__cell${count ? " has-docs" : ""}`}
              title={count ? `${count} documents` : undefined}
            >
              {i + 1}
              {count > 0 && <span className="calendar__count">{count}</span>}
            </span>
          );
        })}
      </div>
    </div>
  );
}

function Heatmap({ byDay }: { byDay: Map<string, TreeEntry[]> }) {
  const days = [...byDay.keys()].sort();
  const first = days[0] ?? TODAY;
  const last = days[days.length - 1] ?? first;
  const max = Math.max(1, ...[...byDay.values()].map((v) => v.length));
  const cells: { day: string; count: number }[] = [];
  const cursor = new Date(`${first}T00:00:00Z`);
  const end = new Date(`${last}T00:00:00Z`);
  while (cursor <= end) {
    const day = cursor.toISOString().slice(0, 10);
    cells.push({ day, count: byDay.get(day)?.length ?? 0 });
    cursor.setUTCDate(cursor.getUTCDate() + 1);
  }
  return (
    <div className="heatmap" role="img" aria-label="Documents per day">
      {cells.map((c) => (
        <span
          key={c.day}
          className="heatmap__cell"
          style={{ opacity: c.count ? 0.25 + (0.75 * c.count) / max : 0.06 }}
          title={`${c.day}: ${c.count}`}
        />
      ))}
    </div>
  );
}

/** Test-only stand-in: the same panels as a static two-column layout. */
function TimelineFallback() {
  const { byDay, month, setMonth } = useTimelineData();
  return (
    <div className="timeline" data-testid="timeline">
      <div className="timeline__left">
        <Calendar byDay={byDay} month={month} onMonth={setMonth} />
        <Heatmap byDay={byDay} />
      </div>
      <div className="timeline__right">
        <ListPanel />
      </div>
    </div>
  );
}
