import { fetchTree, type TreeEntry } from "./api";
import { buildWikiIndex, wikiIdFromPath, type WikiIndex } from "./wikiLinks";

export interface MapIndexEntry {
  id: string;
  path: string;
  title: string;
}

let cachedTree: Promise<TreeEntry[]> | null = null;

function loadTree(): Promise<TreeEntry[]> {
  cachedTree ??= fetchTree()
    .then((res) => res.entries)
    .catch((err: unknown) => {
      cachedTree = null;
      throw err;
    });
  return cachedTree;
}

/** Load (once) the wiki id/title index derived from /api/tree. */
export function loadWikiIndex(): Promise<WikiIndex> {
  return loadTree().then(buildWikiIndex);
}

/** Index `.map.md` documents by map id, doc path and wiki id. */
export function buildMapIndex(entries: TreeEntry[]): Map<string, MapIndexEntry> {
  const index = new Map<string, MapIndexEntry>();
  for (const e of entries) {
    if (!e.isMap) continue;
    const id = e.mapId ?? wikiIdFromPath(e.path);
    const entry: MapIndexEntry = { id, path: e.path, title: e.title ?? id };
    index.set(id, entry);
    index.set(e.path, entry);
    index.set(wikiIdFromPath(e.path), entry);
  }
  return index;
}

/** Load (once) the map document index derived from /api/tree. */
export function loadMapIndex(): Promise<Map<string, MapIndexEntry>> {
  return loadTree().then(buildMapIndex);
}

/** Test-only cache reset. */
export function resetWikiIndexCache(): void {
  cachedTree = null;
}
