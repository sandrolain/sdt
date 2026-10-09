import { createStore } from "./store";

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

const store = createStore<ActiveSection>({
  initial: EMPTY,
  equal: (a, b) => a.path === b.path && a.key === b.key,
});

/** Publish the active heading for a document path. */
export function setActiveSection(path: string, key: string | null): void {
  store.set({ path, key });
}

/** Clear the active heading, but only for the document that owns it. */
export function clearActiveSection(path: string): void {
  if (store.get().path !== path) return;
  store.set(EMPTY);
}

/** Test/reset helper. */
export function resetActiveSection(): void {
  store.reset(EMPTY);
}

/** Heading currently published for `path`, or null when none is. */
export function activeSectionKey(path: string): string | null {
  const current = store.get();
  return current.path === path ? current.key : null;
}

/** Active heading for the current document (empty when none). */
export function useActiveSection(): ActiveSection {
  return store.use();
}
