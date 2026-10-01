import { useSyncExternalStore } from "react";

/**
 * "Focus the tree" request (analysis N7): the TopBar/App shortcut raises it, the
 * mounted tree answers it. A counter rather than a boolean, so two presses in a
 * row both land.
 */

let requests = 0;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

/** Ask whichever tree is mounted to take focus. */
export function requestTreeFocus(): void {
  requests += 1;
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): number {
  return requests;
}

/** Number of focus requests raised so far (test/reset helper included). */
export function useTreeFocusRequests(): number {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}

/** Test/reset helper. */
export function resetTreeFocusRequests(): void {
  requests = 0;
  emit();
}
