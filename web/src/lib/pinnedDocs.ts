import { useSyncExternalStore } from "react";

/** Paths pinned to the top of the tree, in pin order (newest first). */
const KEY = "sdt-pinned-docs";
const VERSION = 1;

function load(): string[] {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as { version?: number; paths?: unknown };
    if (parsed?.version !== VERSION || !Array.isArray(parsed.paths)) return [];
    return parsed.paths.filter((p): p is string => typeof p === "string");
  } catch {
    return [];
  }
}

let current: string[] = load();
const listeners = new Set<() => void>();

function persist(): void {
  try {
    localStorage.setItem(KEY, JSON.stringify({ version: VERSION, paths: current }));
  } catch {
    // storage full or disabled: persistence is non-essential
  }
}

function emit(): void {
  for (const listener of listeners) listener();
}

export function togglePin(path: string): void {
  current = current.includes(path) ? current.filter((p) => p !== path) : [path, ...current];
  persist();
  emit();
}

export function isPinned(path: string): boolean {
  return current.includes(path);
}

/** Reset (tests and workspace teardown). */
export function resetPinnedDocs(): void {
  current = [];
  persist();
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): string[] {
  return current;
}

export function usePinnedDocs(): string[] {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
