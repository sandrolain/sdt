import { useSyncExternalStore } from "react";

/** A pending "jump to this heading" request raised by the Sections list. */
export interface SectionRequest {
  path: string;
  text: string;
  /** bumped on every request so repeats re-trigger the consumer */
  nonce: number;
}

let current: SectionRequest | null = null;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

/** Ask the document panel to switch to render mode and scroll to a heading. */
export function requestSection(path: string, text: string): void {
  current = { path, text, nonce: (current?.nonce ?? 0) + 1 };
  emit();
}

/** Clear the request once the consumer handled it. */
export function consumeSectionRequest(request: SectionRequest): void {
  if (current !== request) return;
  current = null;
  emit();
}

/** Test/reset helper. */
export function resetSectionRequest(): void {
  current = null;
  emit();
}

/** Current request without subscribing (tests). */
export function getSectionRequest(): SectionRequest | null {
  return current;
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): SectionRequest | null {
  return current;
}

export function useSectionRequest(): SectionRequest | null {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
