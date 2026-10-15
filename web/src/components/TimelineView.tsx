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
import {
  HEATMAP_WINDOW_WEEKS,
  WEEKDAY_LABELS,
  heatmapLayout,
  heatmapOpacity,
  type HeatmapCell,
} from "../lib/heatmap";
import { availableKinds, entryKind, kindColor, kindIcon, kindLabel } from "../lib/kinds";
import type { StatePresetId } from "../lib/statusDot";
import { displayTitle } from "../lib/titles";
import { entryMatchesViewState, statePresetLabel } from "../lib/viewFilter";
import { useDockviewTheme, useDockviewWorkspace } from "../lib/dockviewWorkspace";
import { SkeletonLines } from "./Skeleton";
import { TooltipButton } from "./ui/Tooltip";
import { ViewFilters } from "./ViewFilters";

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
  /** The day picked on the heatmap, or null: it filters the list and moves the
   * calendar to its month, so the three panels tell one story (O4). */
  selectedDay: string | null;
  selectDay: (day: string | null) => void;
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
  const [selectedDay, setSelectedDay] = useState<string | null>(null);
  const selectDay = (day: string | null) => {
    setSelectedDay(day);
    // A picked day moves the calendar to its month: the panels must not disagree
    // about which month is on screen.
    if (day) setMonth(day.slice(0, 7));
  };

  if (error) return <p className="content__empty">Timeline error: {error}</p>;
  if (!entries) return <SkeletonLines count={8} label="Loading timeline" />;

  const data: TimelineData = {
    byDay,
    entries,
    month: selectedMonth,
    setMonth,
    selectedDay,
    selectDay,
  };

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
  const { byDay, selectedDay, selectDay } = useTimelineData();
  return <Heatmap byDay={byDay} selectedDay={selectedDay} onSelectDay={selectDay} />;
}

function ListPanel() {
  const { byDay, entries, month, selectedDay, selectDay } = useTimelineData();
  const [kindFilter, setKindFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<StatePresetId>("all");
  const [page, setPage] = useState(0);

  const kinds = useMemo(() => availableKinds(entries), [entries]);
  const days = useMemo(
    () =>
      [...byDay.keys()]
        .filter((d) => (selectedDay ? d === selectedDay : d.startsWith(month)))
        .sort((a, b) => b.localeCompare(a))
        .map((day) => ({
          day,
          docs: (byDay.get(day) ?? [])
            .filter((e) => kindFilter === "all" || entryKind(e) === kindFilter)
            .filter((e) => entryMatchesViewState(e, statusFilter)),
        }))
        .filter((d) => d.docs.length > 0),
    [byDay, month, kindFilter, statusFilter, selectedDay],
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
        <ViewFilters
          preset={statusFilter}
          onChange={(next) => {
            setStatusFilter(next);
            setPage(0);
          }}
        />
        {selectedDay && (
          <button
            type="button"
            className="timeline__day-chip"
            onClick={() => selectDay(null)}
            aria-label={`Clear the ${formatFieldDateOnly(selectedDay)} selection`}
          >
            <Icon name="event" />
            {formatFieldDateOnly(selectedDay)}
            <Icon name="close" />
          </button>
        )}
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
      <MonthTimeline days={slice} statusFilter={statusFilter} />
    </div>
  );
}

function MonthTimeline({
  days,
  statusFilter,
}: {
  days: { day: string; docs: TreeEntry[] }[];
  statusFilter: StatePresetId;
}) {
  if (days.length === 0) {
    // The unfiltered case keeps its original sentence; a filter that emptied the
    // list says which one, so the reader does not think the month is empty.
    return (
      <p className="content__empty">
        {statusFilter === "all"
          ? "No documents."
          : `No document matches the ${statePresetLabel(statusFilter)} status this month.`}
      </p>
    );
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

function Heatmap({
  byDay,
  selectedDay,
  onSelectDay,
}: {
  byDay: Map<string, TreeEntry[]>;
  selectedDay: string | null;
  onSelectDay: (day: string | null) => void;
}) {
  const grid = useMemo(() => heatmapLayout(byDay), [byDay]);
  if (grid.columns.length === 0) {
    return <p className="content__empty">No dated documents yet.</p>;
  }
  return (
    <div className="heatmap">
      <div className="heatmap__body">
        <div className="heatmap__weekdays" aria-hidden="true">
          {WEEKDAY_LABELS.map((label, index) => (
            <span key={label} className="heatmap__weekday">
              {index % 2 === 0 ? label : ""}
            </span>
          ))}
        </div>
        {/* Not a grid role: the cells are buttons with their own names, and a grid
            whose cells are not gridcells would misdescribe the widget. */}
        <div className="heatmap__grid" role="group" aria-label="Documents per day">
          <div className="heatmap__months" aria-hidden="true">
            {grid.columns.map((column, index) => (
              <span key={index} className="heatmap__month">
                {column.monthLabel ?? ""}
              </span>
            ))}
          </div>
          <div className="heatmap__weeks">
            {grid.columns.map((column, index) => (
              <div key={index} className="heatmap__week">
                {column.cells.map((cell, slot) =>
                  cell === null ? (
                    <span key={slot} className="heatmap__cell heatmap__cell--empty" />
                  ) : (
                    <TooltipButton
                      key={slot}
                      label={`${cell.day}: ${cell.count}`}
                      tooltip={heatmapCellTooltip(cell)}
                      className={`heatmap__cell${selectedDay === cell.day ? " is-selected" : ""}`}
                      style={{ opacity: heatmapOpacity(cell.count, grid.max) }}
                      onPress={() => onSelectDay(selectedDay === cell.day ? null : cell.day)}
                    />
                  ),
                )}
              </div>
            ))}
          </div>
        </div>
      </div>
      <div className="heatmap__footer">
        <span className="heatmap__legend" aria-hidden="true">
          <span>less</span>
          {[1, 2, 3, 4].map((step) => (
            <span
              key={step}
              className="heatmap__cell heatmap__swatch"
              style={{ opacity: heatmapOpacity(step, 4) }}
            />
          ))}
          <span>more</span>
        </span>
        <span className="heatmap__caption">
          {grid.capped
            ? `the most recent ${HEATMAP_WINDOW_WEEKS} weeks`
            : "every day the corpus covers"}
        </span>
      </div>
    </div>
  );
}

/** A cell's tooltip: the date, the count and the day's first documents. */
function heatmapCellTooltip(cell: HeatmapCell): string {
  const date = formatFieldDateOnly(cell.day);
  if (cell.count === 0) return `${date} · no documents`;
  const rest = cell.count - cell.titles.length;
  const more = rest > 0 ? ` (+${rest} more)` : "";
  return `${date} · ${cell.count} document${cell.count === 1 ? "" : "s"}: ${cell.titles.join(", ")}${more}`;
}

/** Test-only stand-in: the same panels as a static two-column layout. */
function TimelineFallback() {
  const { byDay, month, setMonth, selectedDay, selectDay } = useTimelineData();
  return (
    <div className="timeline" data-testid="timeline">
      <div className="timeline__left">
        <Calendar byDay={byDay} month={month} onMonth={setMonth} />
        <Heatmap byDay={byDay} selectedDay={selectedDay} onSelectDay={selectDay} />
      </div>
      <div className="timeline__right">
        <ListPanel />
      </div>
    </div>
  );
}
