import { createStore } from "./store";

/** A pending "jump to this heading" request raised by the Sections list. */
export interface SectionRequest {
  path: string;
  text: string;
  /** bumped on every request so repeats re-trigger the consumer */
  nonce: number;
}

const store = createStore<SectionRequest | null>({ initial: null });

/** Ask the document panel to switch to render mode and scroll to a heading. */
export function requestSection(path: string, text: string): void {
  store.set({ path, text, nonce: (store.get()?.nonce ?? 0) + 1 });
}

/** Clear the request once the consumer handled it. */
export function consumeSectionRequest(request: SectionRequest): void {
  if (store.get() !== request) return;
  store.set(null);
}

/** Test/reset helper. */
export function resetSectionRequest(): void {
  store.reset(null);
}

/** Current request without subscribing (tests). */
export function getSectionRequest(): SectionRequest | null {
  return store.get();
}

export function useSectionRequest(): SectionRequest | null {
  return store.use();
}
