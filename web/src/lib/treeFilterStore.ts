import { useSyncExternalStore } from "react";

/** Tree filter selection: the document states the tree shows, and whether the
 *  kind folders nest their entries. The selection is stored as its complement
 *  (the hidden states), so the default — nothing hidden — is today's default
 *  tree and no selection can empty the tree by accident. */
export interface TreeFilterState {
  hiddenStates: string[];
  grouped: boolean;
}

const DEFAULT: TreeFilterState = { hiddenStates: [], grouped: false };

let current: TreeFilterState = DEFAULT;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

/** Replace the set of hidden states; the caller passes the complement of what
 *  the control shows selected. */
export function setHiddenStates(states: string[]): void {
  current = { hiddenStates: states, grouped: current.grouped };
  emit();
}

/** Toggle the grouping of the kind folders into sub-folders. Grouping off
 *  flattens them; the wiki folder hierarchy mirrors the corpus and is kept. */
export function toggleGrouped(): void {
  current = { hiddenStates: current.hiddenStates, grouped: !current.grouped };
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
