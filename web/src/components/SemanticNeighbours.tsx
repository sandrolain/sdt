/**
 * SemanticNeighbours — the semantic tab of the document metadata panel.
 *
 * Reads the same `/api/semantic/graph` payload as the map view and shows the
 * open document's ranked semantic neighbours. Advisory and read-only: it never
 * writes links into the document and degrades to an empty state when the index
 * (or the document) is outside the scoped map.
 */
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Icon } from "../lib/icon";
import { kindColor, kindFromPath, kindIcon } from "../lib/kinds";
import { fetchSemanticGraph, semanticNeighboursFor, type SemanticGraph } from "../lib/semanticMap";
import { useReloadToken } from "../lib/useReloadToken";

export function SemanticNeighbours({ path }: { path: string }) {
  const [graph, setGraph] = useState<SemanticGraph | null>(null);
  const reloadToken = useReloadToken();

  useEffect(() => {
    let alive = true;
    fetchSemanticGraph()
      .then((g) => {
        if (alive) setGraph(g);
      })
      .catch(() => {
        if (alive) setGraph({ nodes: [], edges: [] });
      });
    return () => {
      alive = false;
    };
  }, [reloadToken]);

  const neighbours = useMemo(
    () => (graph ? semanticNeighboursFor(graph, path) : []),
    [graph, path],
  );

  if (!graph) return <p className="content__empty">Loading…</p>;
  if (neighbours.length === 0) {
    return <p className="content__empty">No semantic neighbours.</p>;
  }
  return (
    <ul className="meta-links" role="list">
      {neighbours.map((n) => {
        const kind = kindFromPath(n.path);
        return (
          <li key={n.path}>
            <Link className="meta-link" to={`/docs/${n.path}`} title={n.summary}>
              <Icon
                name={kindIcon(kind)}
                className="meta-link__icon"
                style={{ color: kindColor(kind) }}
                label={n.kind ?? kind}
              />
              {n.title}
              <span className="meta-link__score">{n.score.toFixed(2)}</span>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
