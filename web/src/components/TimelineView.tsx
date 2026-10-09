/**
 * TimelineView — the `/docs/timeline` surface: the corpus over time in one
 * layout. The left column holds a month calendar with a year heatmap beneath it;
 * the right column shows the selected month's documents, grouped by day.
 */
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import type { TreeEntry } from "../lib/api";
import { useCorpusEntries } from "../lib/corpusView";
import { formatFieldDateOnly } from "../lib/frontmatter";
import { Icon } from "../lib/icon";
import { entryKind, kindColor, kindIcon, kindLabel } from "../lib/kinds";
import { displayTitle } from "../lib/titles";
import { SkeletonLines } from "./Skeleton";

/** A stable "today" for the empty-corpus fallback (never called during render). */
const TODAY = new Date().toISOString().slice(0, 10);

/** The ISO day (YYYY-MM-DD) a document is dated on, or null. */
function entryDay(entry: TreeEntry): string | null {
  const raw = entry.modified ?? entry.created;
  return raw ? raw.slice(0, 10) : null;
}

export function TimelineView() {
  const { entries, error } = useCorpusEntries();
  const [month, setMonth] = useState<string>("");

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

  return (
    <div className="timeline">
      <div className="timeline__left">
        <Calendar byDay={byDay} month={selectedMonth} onMonth={setMonth} />
        <Heatmap byDay={byDay} />
      </div>
      <div className="timeline__right">
        <MonthTimeline byDay={byDay} month={selectedMonth} />
      </div>
    </div>
  );
}

function MonthTimeline({ byDay, month }: { byDay: Map<string, TreeEntry[]>; month: string }) {
  const days = [...byDay.keys()]
    .filter((d) => d.startsWith(month))
    .sort((a, b) => b.localeCompare(a));
  if (days.length === 0) {
    return <p className="content__empty">No documents in {month || "this month"}.</p>;
  }
  return (
    <div className="timeline__list">
      {days.map((day) => (
        <section key={day} className="timeline__day">
          <h3 className="timeline__date">{formatFieldDateOnly(day)}</h3>
          <ul className="timeline__docs" role="list">
            {byDay.get(day)!.map((entry) => (
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
