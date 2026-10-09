import { useEffect, useState } from "react";
import type { TreeEntry } from "./api";
import { loadCorpusIndex } from "./corpusIndex";
import { useReloadToken } from "./useReloadToken";

/**
 * All corpus entries for a full-page view. One `/api/tree` fetch, shared and
 * cached by `loadCorpusIndex`, re-run when the corpus changes.
 */
export function useCorpusEntries(): { entries: TreeEntry[] | null; error: string | null } {
  const [entries, setEntries] = useState<TreeEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const reloadToken = useReloadToken();

  useEffect(() => {
    let alive = true;
    loadCorpusIndex()
      .then((index) => {
        if (alive) setEntries(Array.from(index.values()).map((info) => info.entry));
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      alive = false;
    };
  }, [reloadToken]);

  return { entries, error };
}
