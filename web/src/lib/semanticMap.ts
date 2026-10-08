/**
 * Semantic map data: the read-only `/api/semantic/graph` payload and its
 * adaptation to the ported graph engine.
 *
 * This is the **docs** semantic map: it covers the non-wiki knowledge kinds
 * (notes/analysis/decision/architecture/research). The wiki graph is a separate
 * view and the two datasets are never blended.
 */
import { fetchJSON } from "./fetchJson";
import { clusterPalette, DEFAULT_NODE_COLOR, type GraphNode } from "./graphModel";
import type { GraphLinkInput, GraphNodeInput } from "./graph/types";

export interface SemanticNode {
  path: string;
  kind?: string;
  title?: string;
  summary?: string;
}

export interface SemanticEdge {
  from: string;
  to: string;
  score: number;
}

export interface SemanticGraph {
  nodes: SemanticNode[];
  edges: SemanticEdge[];
}

// Categorical identity, not a theme role (phase 9): a semantic proximity edge is
// a distinct identity from the authored wiki relations, so its colour is a raw
// palette value rather than a semantic token.
export const SEMANTIC_EDGE_COLOR = "#cba6f7";

export interface AdaptedSemanticGraph {
  nodes: GraphNodeInput[];
  links: GraphLinkInput[];
  palette: Map<string, string>;
}

/** Map the API payload onto the engine's generic `{nodes, links}` contract. */
export function adaptSemanticGraph(graph: SemanticGraph): AdaptedSemanticGraph {
  const nodes = Array.isArray(graph?.nodes) ? graph.nodes : [];
  const edges = Array.isArray(graph?.edges) ? graph.edges : [];
  const palette = clusterPalette(
    nodes.map(
      (n) => ({ id: n.path, title: n.title ?? n.path, type: n.kind }) as unknown as GraphNode,
    ),
    "type",
  );
  const mapped: GraphNodeInput[] = nodes.map((n) => {
    const group = n.kind || "unknown";
    return {
      id: n.path,
      label: n.title || n.path,
      group,
      color: palette.get(group) ?? DEFAULT_NODE_COLOR,
      description: n.summary,
      path: n.path,
    };
  });
  const ids = new Set(mapped.map((n) => n.id));
  const links: GraphLinkInput[] = edges
    .filter((e) => ids.has(e.from) && ids.has(e.to))
    .map((e) => ({
      source: e.from,
      target: e.to,
      type: "semantic",
      color: SEMANTIC_EDGE_COLOR,
      weight: e.score,
    }));
  return { nodes: mapped, links, palette };
}

export interface SemanticNeighbour {
  path: string;
  title: string;
  kind?: string;
  summary?: string;
  score: number;
}

/**
 * The ranked semantic neighbours of `path` derived from the shared graph edges,
 * so the map view and the panel tab can never rank differently. Empty when the
 * document is not part of the scoped map.
 */
export function semanticNeighboursFor(graph: SemanticGraph, path: string): SemanticNeighbour[] {
  const nodes = Array.isArray(graph?.nodes) ? graph.nodes : [];
  const edges = Array.isArray(graph?.edges) ? graph.edges : [];
  const byPath = new Map(nodes.map((n) => [n.path, n]));
  if (!byPath.has(path)) return [];
  const out: SemanticNeighbour[] = [];
  for (const e of edges) {
    const other = e.from === path ? e.to : e.to === path ? e.from : null;
    if (!other) continue;
    const node = byPath.get(other);
    if (!node) continue;
    out.push({
      path: other,
      title: node.title || other,
      kind: node.kind,
      summary: node.summary,
      score: e.score,
    });
  }
  return out.sort((a, b) => b.score - a.score || a.path.localeCompare(b.path));
}

export function fetchSemanticGraph(): Promise<SemanticGraph> {
  return fetchJSON<SemanticGraph>("/api/semantic/graph");
}
