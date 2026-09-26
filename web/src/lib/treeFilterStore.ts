import { useSyncExternalStore } from "react";

/** Tree filter selection: hide entries whose status is a "done" state, and
 *  group entries into objective (and, for tasks, plan) sub-folders. */
export interface TreeFilterState {
  hideCompleted: boolean;
  grouped: boolean;
}

const DEFAULT: TreeFilterState = { hideCompleted: false, grouped: true };

let current: TreeFilterState = DEFAULT;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

export function toggleHideCompleted(): void {
  current = { hideCompleted: !current.hideCompleted, grouped: current.grouped };
  emit();
}

/** Toggle the grouping of the kind folders into sub-folders. Grouping off
 *  flattens them; the wiki folder hierarchy mirrors the corpus and is kept. */
export function toggleGrouped(): void {
  current = { hideCompleted: current.hideCompleted, grouped: !current.grouped };
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
