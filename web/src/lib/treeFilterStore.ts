import { useSyncExternalStore } from "react";

/** How the tree nests its entries. `flat` renders every document in one list
 *  with no kind folders; `type` groups by document kind with flat contents;
 *  `full` also nests the objective/plan sub-folders inside a kind folder. */
export type GroupMode = "flat" | "type" | "full";

/** Tree filter selection: the document states the tree shows, and how the tree
 *  groups its entries. The selection is stored as its complement (the hidden
 *  states), so the default — nothing hidden — is today's default tree and no
 *  selection can empty the tree by accident. */
export interface TreeFilterState {
  hiddenStates: string[];
  groupMode: GroupMode;
}

const DEFAULT: TreeFilterState = { hiddenStates: [], groupMode: "type" };

let current: TreeFilterState = DEFAULT;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

/** Replace the set of hidden states; the caller passes the complement of what
 *  the control shows selected. */
export function setHiddenStates(states: string[]): void {
  current = { hiddenStates: states, groupMode: current.groupMode };
  emit();
}

/** Replace the grouping mode. The three modes are mutually exclusive, so one
 *  closed value cannot enter an invalid state. */
export function setGroupMode(mode: GroupMode): void {
  current = { hiddenStates: current.hiddenStates, groupMode: mode };
  emit();
}

export function resetTreeFilter(): void {
  current = DEFAULT;
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): TreeFilterState {
  return current;
}

export function useTreeFilter(): TreeFilterState {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
