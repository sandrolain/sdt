import { useSyncExternalStore } from "react";
import type { TreeDir, TreeSortKey } from "./treeSort";

/** Tree sort selection, shared between the list and the tab-bar controls. */
export interface TreeSortState {
  key: TreeSortKey;
}

const DEFAULT: TreeSortState = { key: "created_desc" };

let current: TreeSortState = DEFAULT;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

export function setTreeSortKey(key: TreeSortKey): void {
  if (current.key === key) return;
  current = { ...current, key };
  emit();
}

export function resetTreeSort(): void {
  current = DEFAULT;
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): TreeSortState {
  return current;
}

export function useTreeSort(): TreeSortState {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
