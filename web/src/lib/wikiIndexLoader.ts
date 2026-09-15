import { fetchTree } from "./api";
import { buildWikiIndex, type WikiIndex } from "./wikiLinks";

let cached: Promise<WikiIndex> | null = null;

/** Load (once) the wiki id/title index derived from /api/tree. */
export function loadWikiIndex(): Promise<WikiIndex> {
  cached ??= fetchTree()
    .then((res) => buildWikiIndex(res.entries))
    .catch((err: unknown) => {
      cached = null;
      throw err;
    });
  return cached;
}

/** Test-only cache reset. */
export function resetWikiIndexCache(): void {
  cached = null;
}
