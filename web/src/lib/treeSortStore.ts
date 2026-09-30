import { useSyncExternalStore } from "react";
import { TREE_SORTS, type TreeSortKey } from "./treeSort";
import { loadTreeView, saveTreeView } from "./treeViewStore";

/** Tree sort selection, shared between the list and the tab-bar controls. */
export interface TreeSortState {
  key: TreeSortKey;
}

const DEFAULT: TreeSortState = { key: "created_desc" };

/** The persisted sort key, when it is still one the tree offers. */
function storedKey(): TreeSortKey {
  const stored = loadTreeView().sortKey;
  return TREE_SORTS.some((sort) => sort.id === stored) ? (stored as TreeSortKey) : DEFAULT.key;
}

let current: TreeSortState | null = null;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

/** Live state, seeded from storage on first use so a reload restores the sort. */
function state(): TreeSortState {
  current ??= { key: storedKey() };
  return current;
}

export function setTreeSortKey(key: TreeSortKey): void {
  if (state().key === key) return;
  current = { key };
  saveTreeView({ sortKey: key });
  emit();
}

export function resetTreeSort(): void {
  current = { ...DEFAULT };
  saveTreeView({ sortKey: undefined });
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): TreeSortState {
  return state();
}

export function useTreeSort(): TreeSortState {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
