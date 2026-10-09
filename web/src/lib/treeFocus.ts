import { createStore } from "./store";

/**
 * "Focus the tree" request (analysis N7): the TopBar/App shortcut raises it, the
 * mounted tree answers it. A counter rather than a boolean, so two presses in a
 * row both land.
 */

const store = createStore<number>({ initial: 0 });

/** Ask whichever tree is mounted to take focus. */
export function requestTreeFocus(): void {
  store.set(store.get() + 1);
}

/** Number of focus requests raised so far (test/reset helper included). */
export function useTreeFocusRequests(): number {
  return store.use();
}

/** Test/reset helper. */
export function resetTreeFocusRequests(): void {
  store.reset(0);
}
