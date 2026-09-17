import { useSyncExternalStore } from "react";
import type { TreeDir, TreeSortKey } from "./treeSort";

/** Tree sort selection, shared between the list and the tab-bar controls. */
export interface TreeSortState {
  key: TreeSortKey;
  dir: TreeDir;
}

const DEFAULT: TreeSortState = { key: "created", dir: "desc" };

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

export function setTreeSortDir(dir: TreeDir): void {
  if (current.dir === dir) return;
  current = { ...current, dir };
  emit();
}

export function toggleTreeSortDir(): void {
  setTreeSortDir(current.dir === "asc" ? "desc" : "asc");
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
