import { useSyncExternalStore } from "react";

/**
 * The heading currently in view for a document. A module-level store (rather
 * than React context) so the document panel and the metadata panel — separate
 * dockview panels rendered through portals — can share it without prop drilling.
 */
export interface ActiveSection {
  /** corpus path of the document the heading belongs to */
  path: string;
  /** active heading text, or null when none is in view */
  key: string | null;
}

const EMPTY: ActiveSection = { path: "", key: null };

let current: ActiveSection = EMPTY;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

/** Publish the active heading for a document path. */
export function setActiveSection(path: string, key: string | null): void {
  if (current.path === path && current.key === key) return;
  current = { path, key };
  emit();
}

/** Clear the active heading, but only for the document that owns it. */
export function clearActiveSection(path: string): void {
  if (current.path !== path) return;
  current = EMPTY;
  emit();
}

/** Test/reset helper. */
export function resetActiveSection(): void {
  current = EMPTY;
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): ActiveSection {
  return current;
}

/** Active heading for the current document (empty when none). */
export function useActiveSection(): ActiveSection {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
