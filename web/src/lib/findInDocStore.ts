import { useSyncExternalStore } from "react";

/** Find-in-document bar state (Cmd/Ctrl+F over the open document body). */
export interface FindInDocState {
  open: boolean;
  query: string;
}

const DEFAULT: FindInDocState = { open: false, query: "" };

let current: FindInDocState = DEFAULT;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

export function openFind(): void {
  current = { ...current, open: true };
  emit();
}

export function closeFind(): void {
  current = { ...current, open: false };
  emit();
}

export function setFindQuery(query: string): void {
  current = { ...current, query };
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): FindInDocState {
  return current;
}

export function useFindInDoc(): FindInDocState {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
