/**
 * The heatmap's calendar geometry.
 *
 * The old grid was `repeat(auto-fill, 12px)`: the columns were whatever fitted the
 * width, so a square had no week and no month — a reader could see that some days
 * were darker and nothing else (analysis 20261015-151700, O4). This module turns the
 * day counts into a calendar: columns are weeks, rows are Monday–Sunday, the first
 * day lands on its own weekday, each column carries the month it starts in, and the
 * window is bounded so a years-long corpus stays readable.
 */
import type { TreeEntry } from "./api";

/** Weeks shown at most; a longer span is capped to the most recent window. */
export const HEATMAP_WINDOW_WEEKS = 53;

/** Rows, Monday first — the order the grid draws. */
export const WEEKDAY_LABELS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"] as const;

/** How many documents a cell's tooltip names before it stops listing. */
export const HEATMAP_TOOLTIP_TITLES = 3;

export interface HeatmapCell {
  day: string;
  count: number;
  /** The day's first documents, for the tooltip; the count carries the rest. */
  titles: string[];
}

export interface HeatmapColumn {
  /** The month this column starts in, when it differs from the previous column's. */
  monthLabel: string | null;
  /** Exactly seven slots, Monday first; `null` is a day before the corpus began. */
  cells: (HeatmapCell | null)[];
}

export interface HeatmapGrid {
  columns: HeatmapColumn[];
  /** True when the corpus's span is longer than the window and the grid is capped. */
  capped: boolean;
  /** The window's first and last day; empty strings on an empty input. */
  first: string;
  last: string;
  /** The busiest day's count, which the legend's scale describes. */
  max: number;
}

const DAY_MS = 24 * 60 * 60 * 1000;

/** The UTC date for a `YYYY-MM-DD` string. */
function dayToDate(day: string): Date {
  return new Date(`${day}T00:00:00Z`);
}

/** `YYYY-MM-DD` for a UTC date. */
function dateToDay(date: Date): string {
  return date.toISOString().slice(0, 10);
}

/** The Monday of the week a day belongs to. */
function startOfWeek(day: string): Date {
  const date = dayToDate(day);
  return new Date(date.getTime() - weekdayIndex(day) * DAY_MS);
}

/** Monday-first weekday index (0 = Monday). */
function weekdayIndex(day: string): number {
  return (dayToDate(day).getUTCDay() + 6) % 7;
}

const MONTHS = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
] as const;

/** The short month name of a day, used for the column labels. */
export function monthLabelOf(day: string): string {
  return MONTHS[dayToDate(day).getUTCMonth()];
}

/**
 * The calendar grid for a corpus's documents grouped by day. Deterministic: the same
 * input yields the same columns, the same labels and the same cap, so two loads draw
 * the same picture.
 */
export function heatmapLayout(
  byDay: Map<string, TreeEntry[]>,
  windowWeeks = HEATMAP_WINDOW_WEEKS,
): HeatmapGrid {
  const days = [...byDay.keys()].filter(Boolean).sort();
  if (days.length === 0) {
    return { columns: [], capped: false, first: "", last: "", max: 0 };
  }
  const corpusFirst = days[0];
  const last = days[days.length - 1];

  // The grid runs from the Monday of the corpus's first week to the week of its last
  // day, capped to the most recent `windowWeeks` weeks when that span is longer: a
  // three-day corpus must not draw a year of empty squares.
  const firstMonday = startOfWeek(corpusFirst);
  const lastMonday = startOfWeek(last);
  const totalWeeks = Math.round((lastMonday.getTime() - firstMonday.getTime()) / (7 * DAY_MS)) + 1;
  const capped = totalWeeks > windowWeeks;
  const windowStart = capped
    ? new Date(lastMonday.getTime() - (windowWeeks - 1) * 7 * DAY_MS)
    : firstMonday;
  const first = capped ? dateToDay(windowStart) : corpusFirst;
  const windowStartDate = windowStart;

  const cells: (HeatmapCell | null)[] = [];
  const cursor = new Date(windowStartDate);
  const end = dayToDate(last);
  let max = 0;
  while (cursor <= end) {
    const day = dateToDay(cursor);
    const docs = day >= first ? (byDay.get(day) ?? []) : [];
    if (docs.length === 0 && day < first) {
      cells.push(null);
    } else {
      const titles = docs.slice(0, HEATMAP_TOOLTIP_TITLES).map((doc) => doc.title || doc.path);
      max = Math.max(max, docs.length);
      cells.push({ day, count: docs.length, titles });
    }
    cursor.setUTCDate(cursor.getUTCDate() + 1);
  }

  const columns: HeatmapColumn[] = [];
  let lastMonth: string | null = null;
  for (let i = 0; i < cells.length; i += 7) {
    const week = cells.slice(i, i + 7);
    while (week.length < 7) week.push(null);
    const anchor = week.find((cell) => cell !== null) ?? null;
    const month = anchor ? monthLabelOf(anchor.day) : null;
    const label: string | null = month !== null && month !== lastMonth ? month : null;
    if (label) lastMonth = label;
    columns.push({ monthLabel: label, cells: week });
  }
  return { columns, capped, first, last, max };
}

/** The opacity a cell's count earns on the shared scale (0.25 … 1). */
export function heatmapOpacity(count: number, max: number): number {
  if (count === 0) return 0.06;
  return 0.25 + (0.75 * count) / Math.max(1, max);
}
