import { useEffect, useState } from "react";
import { fetchDoc, type CanvasResponse, type DocResponse, type MermaidResponse } from "./api";
import { useReloadToken } from "./useReloadToken";

export interface DocState {
  doc: DocResponse | CanvasResponse | MermaidResponse | null;
  error: string | null;
  loading: boolean;
}

interface Fetched {
  path: string;
  doc: DocResponse | CanvasResponse | MermaidResponse | null;
  error: string | null;
}

/** Fetch a corpus document by path; state is keyed by path to avoid stale docs. */
export function useDoc(path: string): DocState {
  const [fetched, setFetched] = useState<Fetched | null>(null);
  // a corpus change (a CLI write, an editor save) refetches the body too, so a
  // document-state action taken in the viewer shows its result without a manual
  // reload; the path-keyed state keeps the old doc until the new one lands.
  const reloadToken = useReloadToken();

  useEffect(() => {
    if (path === "") return;
    let alive = true;
    fetchDoc(path)
      .then((d) => {
        if (alive) setFetched({ path, doc: d, error: null });
      })
      .catch((err: unknown) => {
        if (alive) {
          setFetched({ path, doc: null, error: err instanceof Error ? err.message : String(err) });
        }
      });
    return () => {
      alive = false;
    };
  }, [path, reloadToken]);

  const current = path !== "" && fetched?.path === path ? fetched : null;
  return {
    doc: current?.doc ?? null,
    error: current?.error ?? null,
    loading: current === null,
  };
}
