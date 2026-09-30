import { useSyncExternalStore } from "react";

/** Per-document reading position and recency, persisted across reloads. */

/** Namespaced localStorage key holding the positions and the recent list. */
export const READING_KEY = "sdt-reading";

/** Length of the most-recent list; older documents fall out of it. */
export const RECENT_CAP = 12;

/** Upper bound on stored positions, so a long session cannot grow the record. */
export const POSITION_CAP = 100;

/** Minimum gap between two writes while the reader scrolls. */
export const WRITE_THROTTLE_MS = 400;

/** Where the reader stopped in one document. */
export interface ReadingPosition {
  /** px offset of the document scroll container */
  scrollTop: number;
  /** heading text in view at that offset, or null when none was */
  headingId: string | null;
}

export interface ReadingState {
  positions: Record<string, ReadingPosition>;
  /** corpus paths, most recently read first */
  recent: string[];
}

const EMPTY: ReadingState = { positions: {}, recent: [] };

function isPosition(value: unknown): value is ReadingPosition {
  if (typeof value !== "object" || value === null) return false;
  const candidate = value as { scrollTop?: unknown; headingId?: unknown };
  if (typeof candidate.scrollTop !== "number" || !Number.isFinite(candidate.scrollTop))
    return false;
  return candidate.headingId === null || typeof candidate.headingId === "string";
}

/** The stored state, dropping anything that no longer has the right shape. */
export function loadReadingState(): ReadingState {
  try {
    const raw = localStorage.getItem(READING_KEY);
    if (!raw) return EMPTY;
    const parsed = JSON.parse(raw) as { positions?: unknown; recent?: unknown };
    const positions: Record<string, ReadingPosition> = {};
    if (typeof parsed.positions === "object" && parsed.positions !== null) {
      for (const [path, value] of Object.entries(parsed.positions)) {
        if (isPosition(value)) positions[path] = value;
      }
    }
    const recent = Array.isArray(parsed.recent)
      ? parsed.recent.filter((p): p is string => typeof p === "string")
      : [];
    return { positions, recent: recent.slice(0, RECENT_CAP) };
  } catch {
    localStorage.removeItem(READING_KEY);
    return EMPTY;
  }
}

let current: ReadingState | null = null;
let lastWrite = 0;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

/** Live state, seeded from storage on first use so a reload restores the reader. */
function state(): ReadingState {
  current ??= loadReadingState();
  return current;
}

function persist(): void {
  try {
    localStorage.setItem(READING_KEY, JSON.stringify(state()));
  } catch {
    // storage full or disabled: persistence is non-essential
  }
}

/**
 * Store where the reader stopped in `path` and move it to the front of the
 * recent list. Writes are throttled while scrolling (`now` is injectable for
 * tests); call `flushReading` to force the pending state out.
 */
export function recordReading(
  path: string,
  scrollTop: number,
  headingId: string | null,
  now: number = Date.now(),
): void {
  const previous = state().positions[path];
  const next: ReadingPosition = { scrollTop: Math.max(0, Math.round(scrollTop)), headingId };
  const recent = [path, ...state().recent.filter((p) => p !== path)].slice(0, RECENT_CAP);
  if (previous && previous.scrollTop === next.scrollTop && previous.headingId === next.headingId) {
    // same place: only the recency order moved, and it costs no write
    if (state().recent[0] === path) return;
    current = { positions: state().positions, recent };
    lastWrite = now;
    persist();
    emit();
    return;
  }
  const positions = { ...state().positions, [path]: next };
  // evict the oldest position once the record is full, keeping the recent list intact
  for (const oldest of Object.keys(positions)) {
    if (Object.keys(positions).length <= POSITION_CAP) break;
    if (!recent.includes(oldest)) delete positions[oldest];
  }
  current = { positions, recent };
  if (now - lastWrite >= WRITE_THROTTLE_MS) {
    lastWrite = now;
    persist();
  }
  emit();
}

/** Write the current state out, bypassing the throttle (on unmount, pagehide). */
export function flushReading(): void {
  lastWrite = Date.now();
  persist();
}

/** Where the reader stopped in `path`, or null when it was never read. */
export function readingPosition(path: string): ReadingPosition | null {
  return state().positions[path] ?? null;
}

/** Forget every position and the recent list. */
export function clearReadingState(): void {
  current = { positions: {}, recent: [] };
  try {
    localStorage.removeItem(READING_KEY);
  } catch {
    // ignore
  }
  emit();
}

/**
 * Clamp a stored offset to what the document can still scroll: a shorter
 * rebuild (or a narrower panel) would otherwise land the reader past the end.
 */
export function clampScrollTop(
  scrollTop: number,
  scrollHeight: number,
  clientHeight: number,
): number {
  const max = Math.max(0, Math.round(scrollHeight - clientHeight));
  return Math.min(Math.max(0, Math.round(scrollTop)), max);
}

/** Put a document's scroll container back where the reader left it. */
export function restoreScrollOffset(root: HTMLElement, path: string): void {
  const saved = readingPosition(path);
  if (!saved) return;
  const target = clampScrollTop(saved.scrollTop, root.scrollHeight, root.clientHeight);
  if (target > 0) {
    root.scrollTop = target;
    return;
  }
  scrollToHeading(root, saved.headingId);
}

/**
 * Scroll the heading recorded with the position. Used when the offset is gone
 * (a document that never scrolled) or stale (the corpus shrank, so the stored
 * offset no longer describes where the reader was).
 */
export function scrollToHeading(root: HTMLElement, headingId: string | null): boolean {
  if (!headingId) return false;
  const target = Array.from(root.querySelectorAll<HTMLElement>("h1,h2,h3,h4,h5,h6")).find(
    (h) => (h.dataset.heading ?? h.textContent ?? "").trim() === headingId,
  );
  if (!target) return false;
  target.scrollIntoView({ block: "start" });
  return true;
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** The most recently read documents, newest first. */
export function useRecentDocuments(): string[] {
  const recent = (): string[] => state().recent;
  return useSyncExternalStore(subscribe, recent, recent);
}
