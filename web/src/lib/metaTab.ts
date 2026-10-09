import { useSyncExternalStore } from "react";

/** Active tab in the document metadata side panel (info/sections/links/related). */
export type MetaTab = "info" | "sections" | "links" | "related";

const DEFAULT: MetaTab = "info";

let current: MetaTab = DEFAULT;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

export function setMetaTab(tab: MetaTab): void {
  if (tab === current) return;
  current = tab;
  emit();
}

/** Reset to the default tab (tests and workspace teardown). */
export function resetMetaTab(): void {
  if (current === DEFAULT) return;
  current = DEFAULT;
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): MetaTab {
  return current;
}

export function useMetaTab(): MetaTab {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
