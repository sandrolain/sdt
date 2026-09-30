import { useSyncExternalStore } from "react";
import { DEFAULT_FIND_OPTIONS, type FindOptions } from "./findInDoc";

/** Find-in-document bar state (Cmd/Ctrl+F over the open document body). */
export interface FindInDocState {
  open: boolean;
  query: string;
  options: FindOptions;
}

const DEFAULT: FindInDocState = { open: false, query: "", options: { ...DEFAULT_FIND_OPTIONS } };

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

/** Flip one match option; the query and the current match index are kept. */
export function toggleFindOption(option: keyof FindOptions): void {
  current = { ...current, options: { ...current.options, [option]: !current.options[option] } };
  emit();
}

export function resetFindOptions(): void {
  current = { ...current, options: { ...DEFAULT_FIND_OPTIONS } };
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
