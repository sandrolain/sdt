/**
 * SemanticMapView — the `/docs/map` surface.
 *
 * The shared `GraphWorkspace` (the same component as the wiki graph) over the
 * **semantic** data: the non-wiki knowledge corpus, its documents as nodes and
 * their semantic-neighbour edges, read from `/api/semantic/graph`. It passes the
 * raw payload through so the workspace's Semantic-mode control can offer the edge
 * map (default), the 2D projection and the clusters. It degrades to an empty
 * state when no vector snapshot exists.
 */
import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import type { AdaptedEngineGraph } from "../lib/graph/adapter";
import { useOpenDocs } from "../lib/openDocsContext";
import { useReloadToken } from "../lib/useReloadToken";
import { adaptSemanticGraph, fetchSemanticGraph, type SemanticGraph } from "../lib/semantic";
import { GraphWorkspace } from "./GraphWorkspace";

const SEMANTIC_MAP_STORAGE_KEY = "docs-map-v1";

const EMPTY: AdaptedEngineGraph = {
  nodes: [],
  links: [],
  palette: new Map<string, string>(),
  allVerbs: [],
  allKinds: [],
};

export function SemanticMapView() {
  const [data, setData] = useState<SemanticGraph | null>(null);
  const [error, setError] = useState<string | null>(null);
  const reloadToken = useReloadToken();
  const navigate = useNavigate();
  const { open } = useOpenDocs();

  useEffect(() => {
    let alive = true;
    fetchSemanticGraph()
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

  const adapted = useMemo(() => (data ? adaptSemanticGraph(data) : EMPTY), [data]);
  const openDoc = useCallback(
    (id: string) => {
      open(id);
      navigate("/docs");
    },
    [open, navigate],
  );

  return (
    <GraphWorkspace
      adapted={adapted}
      semanticGraph={data ?? undefined}
      storageKey={SEMANTIC_MAP_STORAGE_KEY}
      open={openDoc}
      loading={!data}
      error={error}
      title="Map"
    />
  );
}
