// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import {
  applyReadingPrefs,
  currentReadingPrefs,
  DEFAULT_READING_PREFS,
  FONT_SCALES,
  MEASURE_CH,
  READING_PREFS_KEY,
  readReadingPrefs,
  resetReadingPrefs,
  setFontScale,
  setMeasureOn,
  stepFontScale,
  useReadingPrefs,
} from "./readingPrefs";

function rootVars(): Record<string, string> {
  const style = document.documentElement.style;
  return {
    scale: style.getPropertyValue("--reading-scale"),
    measure: style.getPropertyValue("--reading-measure"),
  };
}

describe("readingPrefs", () => {
  beforeEach(() => {
    localStorage.clear();
    resetReadingPrefs();
    document.documentElement.removeAttribute("style");
  });

  it("defaults to the base size with the measure on", () => {
    expect(currentReadingPrefs()).toEqual(DEFAULT_READING_PREFS);
    expect(DEFAULT_READING_PREFS).toEqual({ fontScale: 1, measureOn: true });
    expect(MEASURE_CH).toBe(72);
  });

  it("publishes the preferences as root custom properties", () => {
    act(() => setFontScale(1.15));
    expect(rootVars().scale).toBe("1.15");
    expect(rootVars().measure).toBe("72ch");
    act(() => setMeasureOn(false));
    expect(rootVars().measure).toBe("100%");
  });

  it("persists the preferences and restores them on a reload", async () => {
    act(() => setFontScale(1.3));
    act(() => setMeasureOn(false));
    expect(JSON.parse(localStorage.getItem(READING_PREFS_KEY) ?? "{}")).toEqual({
      fontScale: 1.3,
      measureOn: false,
    });
    // a reload is a fresh module reading the record back
    vi.resetModules();
    const reloaded = await import("./readingPrefs");
    expect(reloaded.currentReadingPrefs()).toEqual({ fontScale: 1.3, measureOn: false });
  });

  it("falls back to the defaults for a malformed record", () => {
    localStorage.setItem(READING_PREFS_KEY, "{nope");
    expect(readReadingPrefs()).toEqual(DEFAULT_READING_PREFS);
    localStorage.setItem(READING_PREFS_KEY, JSON.stringify({ fontScale: 9, measureOn: "yes" }));
    expect(readReadingPrefs()).toEqual(DEFAULT_READING_PREFS);
  });

  it("steps the size along the scale and clamps at both ends", () => {
    act(() => setFontScale(FONT_SCALES[0]));
    act(() => stepFontScale(-1));
    expect(currentReadingPrefs().fontScale).toBe(FONT_SCALES[0]);
    act(() => stepFontScale(1));
    expect(currentReadingPrefs().fontScale).toBe(FONT_SCALES[1]);
    act(() => stepFontScale(99));
    expect(currentReadingPrefs().fontScale).toBe(FONT_SCALES[FONT_SCALES.length - 1]);
  });

  it("notifies subscribers when a preference changes", () => {
    const { result } = renderHook(() => useReadingPrefs());
    act(() => setFontScale(1.15));
    expect(result.current.fontScale).toBe(1.15);
    act(() => setMeasureOn(false));
    expect(result.current.measureOn).toBe(false);
  });

  it("applies an explicit preference set without going through the store", () => {
    applyReadingPrefs({ fontScale: 0.875, measureOn: true });
    expect(rootVars().scale).toBe("0.875");
  });
});
