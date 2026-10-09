import { createStore } from "./store";

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

function persist(paths: string[]): void {
  try {
    localStorage.setItem(KEY, JSON.stringify({ version: VERSION, paths }));
  } catch {
    // storage full or disabled: persistence is non-essential
  }
}

const store = createStore<string[]>({ initial: load, save: persist });

export function togglePin(path: string): void {
  const current = store.get();
  store.set(current.includes(path) ? current.filter((p) => p !== path) : [path, ...current]);
}

export function isPinned(path: string): boolean {
  return store.get().includes(path);
}

/** Reset (tests and workspace teardown). */
export function resetPinnedDocs(): void {
  store.reset([]);
}

export function usePinnedDocs(): string[] {
  return store.use();
}
