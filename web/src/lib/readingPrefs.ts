import { useSyncExternalStore } from "react";

/** Global reading preferences (adoption decision Q2): text size and measure. */

/** Namespaced localStorage key, like the theme and the tree-view record. */
export const READING_PREFS_KEY = "sdt-reading-prefs";

/** Line length the measure caps the rendered text at, in characters. */
export const MEASURE_CH = 72;

/** Text-size steps, smallest to largest; the scale multiplies the base size. */
export const FONT_SCALES = [0.875, 1, 1.15, 1.3] as const;

export type FontScale = (typeof FONT_SCALES)[number];

export interface ReadingPrefs {
  /** multiplier over the base document size */
  fontScale: FontScale;
  /** cap the line length at MEASURE_CH */
  measureOn: boolean;
}

export const DEFAULT_READING_PREFS: ReadingPrefs = { fontScale: 1, measureOn: true };

function isFontScale(value: unknown): value is FontScale {
  return FONT_SCALES.includes(value as FontScale);
}

/** The stored preferences, or the defaults when absent, stale or malformed. */
export function readReadingPrefs(): ReadingPrefs {
  try {
    const raw = localStorage.getItem(READING_PREFS_KEY);
    if (!raw) return { ...DEFAULT_READING_PREFS };
    const parsed = JSON.parse(raw) as { fontScale?: unknown; measureOn?: unknown };
    return {
      fontScale: isFontScale(parsed.fontScale) ? parsed.fontScale : DEFAULT_READING_PREFS.fontScale,
      measureOn:
        typeof parsed.measureOn === "boolean" ? parsed.measureOn : DEFAULT_READING_PREFS.measureOn,
    };
  } catch {
    return { ...DEFAULT_READING_PREFS };
  }
}

let current: ReadingPrefs | null = null;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

/** Live preferences, seeded from storage on first use. */
function state(): ReadingPrefs {
  current ??= readReadingPrefs();
  return current;
}

/** The preferences in effect, for the boot effect that publishes them. */
export function currentReadingPrefs(): ReadingPrefs {
  return state();
}

function persist(prefs: ReadingPrefs): void {
  try {
    localStorage.setItem(READING_PREFS_KEY, JSON.stringify(prefs));
  } catch {
    // storage full or disabled: the preference still applies to this session
  }
}

/**
 * Publish the preferences as CSS custom properties on the document root, so
 * every rendered document reflows without a reload and without a per-tab
 * control: `--reading-scale` drives `--reading-size`, `--reading-measure` caps
 * the line length.
 */
export function applyReadingPrefs(prefs: ReadingPrefs): void {
  const root = document.documentElement;
  root.style.setProperty("--reading-scale", String(prefs.fontScale));
  root.style.setProperty("--reading-measure", prefs.measureOn ? `${MEASURE_CH}ch` : "100%");
  persist(prefs);
}

/** Move one step along the text-size scale (clamped at both ends). */
export function stepFontScale(delta: number): void {
  const current_ = state().fontScale;
  const at = FONT_SCALES.indexOf(current_);
  const next = FONT_SCALES[Math.min(FONT_SCALES.length - 1, Math.max(0, at + delta))];
  setFontScale(next);
}

export function setFontScale(fontScale: FontScale): void {
  if (state().fontScale === fontScale) return;
  current = { ...state(), fontScale };
  applyReadingPrefs(current);
  emit();
}

export function setMeasureOn(measureOn: boolean): void {
  if (state().measureOn === measureOn) return;
  current = { ...state(), measureOn };
  applyReadingPrefs(current);
  emit();
}

/** Forget the stored preferences and return to the defaults. */
export function resetReadingPrefs(): void {
  current = { ...DEFAULT_READING_PREFS };
  applyReadingPrefs(current);
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): ReadingPrefs {
  return state();
}

/** The global reading preferences. */
export function useReadingPrefs(): ReadingPrefs {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
