/**
 * SemanticMapView — the `/docs/map` surface.
 *
 * A standalone, read-only semantic map over the non-wiki knowledge corpus,
 * rendered by the ported three.js graph engine. It is a **separate view** from
 * the wiki graph: the two datasets are never blended. The graph shows documents
 * as nodes (coloured by kind) and their semantic-neighbour edges, both read from
 * `/api/semantic/graph`, and degrades to an empty state when no vector snapshot
 * exists.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { GraphView, type GraphViewHandle } from "./GraphView";
import { SkeletonLines } from "./Skeleton";
import { Icon } from "../lib/icon";
import { useOpenDocs } from "../lib/openDocsContext";
import { useReloadToken } from "../lib/useReloadToken";
import { adaptSemanticGraph, fetchSemanticGraph, type SemanticGraph } from "../lib/semanticMap";

export function SemanticMapView() {
  const [data, setData] = useState<SemanticGraph | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [labels, setLabels] = useState(true);
  const viewRef = useRef<GraphViewHandle | null>(null);
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

  const adapted = useMemo(
    () =>
      data
        ? adaptSemanticGraph(data)
        : { nodes: [], links: [], palette: new Map<string, string>() },
    [data],
  );

  const fit = useCallback(() => viewRef.current?.fitView(), []);
  const openDoc = useCallback(
    (path: string) => {
      open(path);
      navigate("/docs");
    },
    [open, navigate],
  );

  if (error) return <p className="content__empty">Semantic map error: {error}</p>;
  if (!data) {
    return (
      <div className="semantic-map">
        <SkeletonLines count={5} label="Loading semantic map" />
      </div>
    );
  }

  const empty = adapted.nodes.length === 0;

  return (
    <div className="semantic-map" data-testid="semantic-map">
      <header className="semantic-map__bar">
        <h2 className="semantic-map__title">Semantic map</h2>
        <span className="semantic-map__scope">
          notes · analysis · decisions · architecture · research
        </span>
        <div className="semantic-map__actions">
          <button
            type="button"
            className={`semantic-map__button${labels ? " is-active" : ""}`}
            aria-pressed={labels}
            onClick={() => setLabels((v) => !v)}
          >
            <Icon name="label" />
            Labels
          </button>
          <button type="button" className="semantic-map__button" onClick={fit} disabled={empty}>
            <Icon name="center_focus_strong" />
            Fit
          </button>
        </div>
      </header>
      {empty ? (
        <p className="content__empty semantic-map__empty">
          No semantic map yet. Run <code>sdt context search --semantic</code> once to build the
          vector snapshot.
        </p>
      ) : (
        <div className="semantic-map__canvas">
          <GraphView
            ref={viewRef}
            nodes={adapted.nodes}
            links={adapted.links}
            mode="2d"
            layout="force"
            selectedId={selectedId}
            onSelect={setSelectedId}
            labels={labels ? "auto" : "none"}
            onNodeDoubleClick={(node) => openDoc(String(node.path ?? node.id))}
          />
        </div>
      )}
    </div>
  );
}
