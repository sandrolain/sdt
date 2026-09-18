import { useEffect, useState } from "react";
import { fetchTree, type TreeEntry } from "./api";
import { entryKind, kindFromPath, type EntryFilterKind } from "./kinds";

/** Full tree entry (plus derived kind) for one corpus path, from `/api/tree`. */
export interface CorpusInfo {
  entry: TreeEntry;
  kind: EntryFilterKind;
}

export type CorpusIndex = Map<string, CorpusInfo>;

let cache: Promise<CorpusIndex> | null = null;

/** Drop the cached index (tests / live reload). */
export function resetCorpusIndexCache(): void {
  cache = null;
}

/** Load the corpus path → kind index once, shared by every consumer. */
export function loadCorpusIndex(): Promise<CorpusIndex> {
  if (!cache) {
    cache = fetchTree()
      .then((res) => {
        const map: CorpusIndex = new Map();
        for (const entry of res.entries ?? []) {
          map.set(entry.path, { entry, kind: entryKind(entry) });
        }
        return map;
      })
      .catch((err: unknown) => {
        cache = null;
        throw err;
      });
  }
  return cache;
}

/** Corpus path behind a docs/wiki hash href, or null for external/other links. */
export function hrefCorpusPath(href: string): string | null {
  if (href.startsWith("/docs/")) return decodeURIComponent(href.slice("/docs/".length));
  if (href.startsWith("/wiki/")) return `context/wiki/${href.slice("/wiki/".length)}.md`;
  return null;
}

/** Kind of a link target; falls back to the folder when the index lacks it. */
export function linkKind(href: string, index?: CorpusIndex): EntryFilterKind | undefined {
  const path = hrefCorpusPath(href);
  if (!path) return undefined;
  return index?.get(path)?.kind ?? kindFromPath(path);
}

/** Reactive kind for a corpus path, seeded by the folder fallback. */
export function useCorpusKind(path: string): EntryFilterKind {
  const [kind, setKind] = useState<EntryFilterKind>(() => kindFromPath(path));
  useEffect(() => {
    let alive = true;
    loadCorpusIndex()
      .then((index) => {
        if (alive) setKind(index.get(path)?.kind ?? kindFromPath(path));
      })
      .catch(() => {
        // keep the folder fallback
      });
    return () => {
      alive = false;
    };
  }, [path]);
  return kind;
}
