/**
 * The status filter the Gallery, the Timeline and the Kanban share.
 *
 * The vocabulary is not defined here: it is `lib/statusDot.ts`, the same registry
 * the Documents tree's toolbar reads, so no view can disagree with the tree about
 * what "open" means. This module only turns a preset into a predicate — and states
 * the honest edge: an entry with no status at all (`no-state`) belongs to no family,
 * so it passes `All` and is absent from `Open`, `Closed` and `Deferred`.
 */
import { STATE_PRESETS, entryState, type StateKey, type StatePresetId } from "./statusDot";
import type { TreeEntry } from "./api";

const NO_PLANS = new Map<string, TreeEntry[]>();
const NO_TASKS = new Map<string, TreeEntry[]>();

/** The state keys one preset admits; `all` admits every key, including `no-state`. */
export function presetKeys(id: StatePresetId): readonly StateKey[] {
  return STATE_PRESETS.find((preset) => preset.id === id)?.keys ?? [];
}

/** True when a state key passes the preset. */
export function matchesViewState(id: StatePresetId, key: StateKey): boolean {
  return presetKeys(id).includes(key);
}

/** True when an entry passes the preset, by its effective state. */
export function entryMatchesViewState(entry: TreeEntry, id: StatePresetId): boolean {
  return matchesViewState(id, entryState(entry, NO_PLANS, NO_TASKS).key);
}

/** The preset's label, for a control and for an empty state that names it. */
export function statePresetLabel(id: StatePresetId): string {
  return STATE_PRESETS.find((preset) => preset.id === id)?.label ?? id;
}

/** The presets the control offers, in registry order. */
export function statePresetOptions(): { id: StatePresetId; label: string }[] {
  return STATE_PRESETS.map((preset) => ({ id: preset.id, label: preset.label }));
}
