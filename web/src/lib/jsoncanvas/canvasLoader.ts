/**
 * Lazy, cached loader for an external `.canvas` document referenced by a board
 * node (`file`/`link`). Only the level currently shown fetches its direct
 * children; the cache makes re-entering a level instant. Mirrors the
 * `wikiIndexLoader` module-cache pattern.
 */
import { fetchWikiBoard } from "../api";
import { normalizeCanvas, type CanvasDocument } from "./document";

const cache = new Map<string, Promise<CanvasDocument | null>>();

/** Load (once) a `.canvas` document by path; null when missing/unreadable. */
export function loadCanvas(path: string): Promise<CanvasDocument | null> {
  const existing = cache.get(path);
  if (existing) return existing;
  const pending = fetchWikiBoard(path)
    .then((res) => normalizeCanvas(res))
    .catch(() => {
      cache.delete(path);
      return null;
    });
  cache.set(path, pending);
  return pending;
}

/** Test-only cache reset. */
export function resetCanvasCache(): void {
  cache.clear();
}
