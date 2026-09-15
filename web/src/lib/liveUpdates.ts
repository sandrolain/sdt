import { clearPreviewCache } from "./preview";
import { resetWikiIndexCache } from "./wikiIndexLoader";

/** Window event dispatched after a live corpus change invalidates caches. */
export const LIVE_RELOAD_EVENT = "sdt:reload";

export interface LiveChange {
  paths: string[];
}

export type LiveHandler = (change: LiveChange) => void;

/** Subscribe to `/api/events`; no-op (and returns a no-op) without EventSource. */
export function connectLiveUpdates(onChange: LiveHandler): () => void {
  if (typeof EventSource === "undefined") return () => undefined;
  const source = new EventSource("/api/events");
  const listener = (event: MessageEvent) => {
    try {
      const data = JSON.parse(event.data) as Partial<LiveChange>;
      onChange({ paths: Array.isArray(data.paths) ? data.paths : [] });
    } catch {
      // ignore malformed events
    }
  };
  source.addEventListener("change", listener as EventListener);
  return () => {
    source.removeEventListener("change", listener as EventListener);
    source.close();
  };
}

/** Invalidate cached corpus data and notify listeners to refetch. */
export function applyLiveChange(): void {
  resetWikiIndexCache();
  clearPreviewCache();
  if (typeof window !== "undefined") window.dispatchEvent(new Event(LIVE_RELOAD_EVENT));
}
