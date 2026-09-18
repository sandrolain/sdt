import { useSyncExternalStore } from "react";

/** Tree filter selection: hide entries whose status is a "done" state. */
export interface TreeFilterState {
  hideCompleted: boolean;
}

const DEFAULT: TreeFilterState = { hideCompleted: false };

let current: TreeFilterState = DEFAULT;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

export function toggleHideCompleted(): void {
  current = { hideCompleted: !current.hideCompleted };
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
