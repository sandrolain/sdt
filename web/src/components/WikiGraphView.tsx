/**
 * WikiGraphView — the `/wiki/graph` surface: the shared `GraphWorkspace` over
 * the wiki graph data (analysis B1/B2/D1/D2/D4).
 */
import { useCallback, useEffect, useMemo, useState } from "react";
import { adaptToEngine, type AdaptedEngineGraph } from "../lib/graph/adapter";
import { fetchWikiGraph, type GraphData } from "../lib/graphModel";
import type { ClusterKey } from "../lib/graphModel";
import { useOpenDocs } from "../lib/openDocsContext";
import { useReloadToken } from "../lib/useReloadToken";
import { GraphWorkspace } from "./GraphWorkspace";

export const WIKI_GRAPH_STORAGE_KEY = "wiki-graph-v2";

const EMPTY: AdaptedEngineGraph = {
  nodes: [],
  links: [],
  palette: new Map<string, string>(),
  allVerbs: [],
  allKinds: [],
};

export function WikiGraphView() {
  const [data, setData] = useState<GraphData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const reloadToken = useReloadToken();
  const { open: openDoc } = useOpenDocs();

  useEffect(() => {
    let alive = true;
    fetchWikiGraph()
      .then((d) => {
        if (alive) setData(d);
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      alive = false;
    };
  }, [reloadToken]);

  const adapted = useMemo(
    () => (data ? adaptToEngine(data, { clusterKey: "type" }) : EMPTY),
    [data],
  );
  const recluster = useCallback(
    (clusterKey: ClusterKey) => (data ? adaptToEngine(data, { clusterKey }) : EMPTY),
    [data],
  );
  const open = useCallback((id: string) => openDoc(`context/wiki/${id}.md`), [openDoc]);

  return (
    <GraphWorkspace
      adapted={adapted}
      recluster={recluster}
      storageKey={WIKI_GRAPH_STORAGE_KEY}
      open={open}
      loading={!data}
      error={error}
      title="Graph"
    />
  );
}
