import { describe, expect, it } from "vitest";
import { STATE_KEYS, STATE_PRESETS } from "./statusDot";
import {
  entryMatchesViewState,
  matchesViewState,
  presetKeys,
  statePresetLabel,
  statePresetOptions,
} from "./viewFilter";
import type { TreeEntry } from "./api";

// An analysis carries a status; a notes entry carries none at all, which is the
// `no-state` edge this filter states rather than hides.
const entry = (status?: string, kind = "analysis"): TreeEntry =>
  ({ path: `context/${kind}/a.md`, kind, title: "A", status }) as TreeEntry;

describe("the view status filter", () => {
  it("reads its presets from the status registry, not from a second list", () => {
    expect(statePresetOptions()).toEqual(
      STATE_PRESETS.map((preset) => ({ id: preset.id, label: preset.label })),
    );
    expect(statePresetLabel("open")).toBe("Open");
  });

  it("admits every key under All, including a state-less entry", () => {
    for (const key of STATE_KEYS) expect(matchesViewState("all", key)).toBe(true);
    expect(matchesViewState("all", "no-state")).toBe(true);
  });

  it("filters by family for the other presets, and states the honest edge", () => {
    expect(matchesViewState("open", "active")).toBe(true);
    expect(matchesViewState("open", "completed")).toBe(false);
    expect(matchesViewState("closed", "completed")).toBe(true);
    expect(matchesViewState("closed", "archived")).toBe(true);
    // A document with no status at all belongs to no family: it is listed under All
    // and absent from the family presets, which is what the control says it does.
    expect(matchesViewState("open", "no-state")).toBe(false);
    expect(matchesViewState("closed", "no-state")).toBe(false);
    expect(matchesViewState("deferred", "no-state")).toBe(false);
  });

  it("narrows an entry by its effective state", () => {
    expect(entryMatchesViewState(entry("active"), "open")).toBe(true);
    expect(entryMatchesViewState(entry("active"), "closed")).toBe(false);
    expect(entryMatchesViewState(entry("completed"), "closed")).toBe(true);
    expect(entryMatchesViewState(entry("postponed"), "deferred")).toBe(true);
    // A kind with no status vocabulary is `no-state`: listed under All, absent from
    // every family preset — the edge this filter states instead of hiding.
    expect(entryMatchesViewState(entry(undefined, "notes"), "all")).toBe(true);
    expect(entryMatchesViewState(entry(undefined, "notes"), "open")).toBe(false);
    expect(entryMatchesViewState(entry(undefined, "notes"), "closed")).toBe(false);
  });

  it("has no preset without keys, and every key belongs to some family or none", () => {
    for (const preset of STATE_PRESETS) {
      expect([preset.id, presetKeys(preset.id)]).toEqual([preset.id, preset.keys]);
    }
    expect(presetKeys("all").length).toBeGreaterThan(0);
  });
});
