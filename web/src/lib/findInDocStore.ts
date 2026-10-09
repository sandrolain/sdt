import { createStore } from "./store";
import { DEFAULT_FIND_OPTIONS, type FindOptions } from "./findInDoc";

/** Find-in-document bar state (Cmd/Ctrl+F over the open document body). */
export interface FindInDocState {
  open: boolean;
  query: string;
  options: FindOptions;
}

const DEFAULT: FindInDocState = { open: false, query: "", options: { ...DEFAULT_FIND_OPTIONS } };

const store = createStore<FindInDocState>({ initial: DEFAULT });

export function openFind(): void {
  store.set({ ...store.get(), open: true });
}

export function closeFind(): void {
  store.set({ ...store.get(), open: false });
}

export function setFindQuery(query: string): void {
  store.set({ ...store.get(), query });
}

/**
 * Clear the query on a document switch; the match options and the open state
 * survive, so the bar keeps its configuration and stays visible.
 */
export function resetFindQuery(): void {
  store.set({ ...store.get(), query: "" });
}

/** Flip one match option; the query and the current match index are kept. */
export function toggleFindOption(option: keyof FindOptions): void {
  const current = store.get();
  store.set({ ...current, options: { ...current.options, [option]: !current.options[option] } });
}

export function resetFindOptions(): void {
  store.set({ ...store.get(), options: { ...DEFAULT_FIND_OPTIONS } });
}

export function useFindInDoc(): FindInDocState {
  return store.use();
}
