import { useSyncExternalStore } from "react";
import { loadTreeView, saveTreeView } from "./treeViewStore";

/** How the tree nests its entries. `flat` renders every document in one list
 *  with no kind folders; `type` groups by document kind with flat contents;
 *  `full` also nests the objective/plan sub-folders inside a kind folder. */
export type GroupMode = "flat" | "type" | "full";

/** Every grouping mode, in toolbar order — the single list the value validates against. */
export const GROUP_MODES: GroupMode[] = ["flat", "type", "full"];

/** Tree filter selection: the document states the tree shows, and how the tree
 *  groups its entries. The selection is stored as its complement (the hidden
 *  states), so the default — nothing hidden — is today's default tree and no
 *  selection can empty the tree by accident. */
export interface TreeFilterState {
  hiddenStates: string[];
  groupMode: GroupMode;
  /** Hide kind folders with no visible document (a decluttered view). */
  hideEmpty: boolean;
}

const DEFAULT: TreeFilterState = { hiddenStates: [], groupMode: "type", hideEmpty: false };

/** The persisted slices, each dropped when it no longer has the right shape. */
function storedState(): TreeFilterState {
  const stored = loadTreeView();
  const hidden = stored.hiddenStates;
  const mode = stored.groupMode;
  return {
    hiddenStates: Array.isArray(hidden)
      ? hidden.filter((s): s is string => typeof s === "string")
      : DEFAULT.hiddenStates,
    groupMode: GROUP_MODES.includes(mode as GroupMode) ? (mode as GroupMode) : DEFAULT.groupMode,
    hideEmpty: typeof stored.hideEmpty === "boolean" ? stored.hideEmpty : DEFAULT.hideEmpty,
  };
}

let current: TreeFilterState | null = null;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

/** Live state, seeded from storage on first use so a reload restores the filter. */
function state(): TreeFilterState {
  current ??= storedState();
  return current;
}

/** Replace the set of hidden states; the caller passes the complement of what
 *  the control shows selected. */
export function setHiddenStates(states: string[]): void {
  current = { hiddenStates: states, groupMode: state().groupMode, hideEmpty: state().hideEmpty };
  saveTreeView({ hiddenStates: states });
  emit();
}

/** Replace the grouping mode. The three modes are mutually exclusive, so one
 *  closed value cannot enter an invalid state. */
export function setGroupMode(mode: GroupMode): void {
  current = { hiddenStates: state().hiddenStates, groupMode: mode, hideEmpty: state().hideEmpty };
  saveTreeView({ groupMode: mode });
  emit();
}

/** Toggle the hide-empty-sections flag. */
export function setHideEmpty(hideEmpty: boolean): void {
  current = { ...state(), hideEmpty };
  saveTreeView({ hideEmpty });
  emit();
}

export function resetTreeFilter(): void {
  current = { ...DEFAULT };
  saveTreeView({ hiddenStates: undefined, groupMode: undefined, hideEmpty: undefined });
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): TreeFilterState {
  return state();
}

export function useTreeFilter(): TreeFilterState {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
