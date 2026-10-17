// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import {
  clampPanelWidth,
  DEFAULT_PANEL_WIDTH,
  MAX_PANEL_WIDTH,
  MIN_PANEL_WIDTH,
  PANEL_WIDTH_KEY,
  readPanelWidth,
  writePanelWidth,
} from "./agentPanelWidth";

afterEach(() => {
  localStorage.clear();
});

describe("clampPanelWidth", () => {
  it("holds a width between the pane's floor and its ceiling", () => {
    expect(clampPanelWidth(MIN_PANEL_WIDTH - 200)).toBe(MIN_PANEL_WIDTH);
    expect(clampPanelWidth(MAX_PANEL_WIDTH + 400)).toBe(MAX_PANEL_WIDTH);
    expect(clampPanelWidth(713.4)).toBe(713);
  });

  it("falls back to the default when the value is not a number", () => {
    expect(clampPanelWidth(Number.NaN)).toBe(DEFAULT_PANEL_WIDTH);
    expect(clampPanelWidth(Number.POSITIVE_INFINITY)).toBe(DEFAULT_PANEL_WIDTH);
  });
});

describe("the remembered width", () => {
  it("is the default until the reader has resized the panel", () => {
    expect(readPanelWidth()).toBe(DEFAULT_PANEL_WIDTH);
  });

  it("is the width the reader left, read back under its own key", () => {
    writePanelWidth(713);
    expect(localStorage.getItem(PANEL_WIDTH_KEY)).toBe("713");
    expect(readPanelWidth()).toBe(713);
  });

  it("clamps on the way in, so a width out of range is never stored as it stands", () => {
    writePanelWidth(MAX_PANEL_WIDTH + 400);
    expect(localStorage.getItem(PANEL_WIDTH_KEY)).toBe(String(MAX_PANEL_WIDTH));
  });

  it("reads a value it did not write as the default rather than as a broken panel", () => {
    localStorage.setItem(PANEL_WIDTH_KEY, "wide");
    expect(readPanelWidth()).toBe(DEFAULT_PANEL_WIDTH);
  });
});
