/**
 * Semantic graph data: the read-only `/api/semantic/graph` payload and its
 * adaptation to the ported graph engine.
 *
 * This is the **docs** semantic map: it covers the non-wiki knowledge kinds
 * (notes/analysis/decision/architecture/research). The wiki graph is a separate
 * view and the two datasets are never blended.
 */
import { fetchJSON } from "../fetchJson";
import type { AdaptedEngineGraph } from "../graph/adapter";
import { clusterPalette, DEFAULT_NODE_COLOR, type GraphNode } from "../graphModel";
import type { GraphLinkInput, GraphNodeInput } from "../graph/types";
import { semanticEdgeBudget, thinEdges } from "./edges";

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
  /** 2D PCA coordinates per path (server-side, deterministic); absent on empty. */
  projection?: Record<string, [number, number]>;
  /** Cluster index per path (server-side, deterministic); absent on empty. */
  clusters?: Record<string, number>;
  /** Cluster name per index, derived server-side from the members it holds; absent
   * on an older payload, in which case the index is shown as `cluster <n>`. */
  clusterLabels?: Record<string, string>;
  /** Set when an on-demand build was attempted and failed. */
  warning?: string;
}

// Categorical identity, not a theme role (phase 9): a semantic proximity edge is
// a distinct identity from the authored wiki relations, so its colour is a raw
// palette value rather than a semantic token.
export const SEMANTIC_EDGE_COLOR = "#cba6f7";

/**
 * The docs-map visualization modes. Distinct from the graph *Layout* selector:
 * `edges` is the shipped edge map (the U5 baseline and the default), `projection`
 * and `clusters` are the U1/U2 opt-ins, and `combined` is edges + clusters.
 */
export type SemanticMode = "edges" | "projection" | "clusters" | "combined";

/** The mode control's options, in display order. Edges first == the default. */
export const SEMANTIC_MODE_OPTIONS: { id: SemanticMode; label: string }[] = [
  { id: "edges", label: "Edges" },
  { id: "projection", label: "Projection" },
  { id: "clusters", label: "Clusters" },
  { id: "combined", label: "Combined" },
];

/** True when the mode positions nodes by the projection (Projection/Combined). */
export function semanticModeProjects(mode: SemanticMode): boolean {
  return mode === "projection" || mode === "combined";
}

/** True when the mode colours/groups nodes by cluster (Clusters/Combined). */
export function semanticModeClusters(mode: SemanticMode): boolean {
  return mode === "clusters" || mode === "combined";
}

// Projection coordinates are rescaled into this half-extent so the engine's
// world space is filled regardless of the PCA's raw scale.
const SEMANTIC_PROJECTION_EXTENT = 260;

/** Rescale the raw projection into a bounded square, preserving relative
 * position. An all-zero projection (degenerate snapshot) maps to the origin. */
function scaledProjection(
  projection: Record<string, [number, number]> | undefined,
): Map<string, [number, number]> {
  const out = new Map<string, [number, number]>();
  const entries = Object.entries(projection ?? {});
  let maxAbs = 0;
  for (const [, xy] of entries) {
    maxAbs = Math.max(maxAbs, Math.abs(xy[0]), Math.abs(xy[1]));
  }
  const scale = maxAbs > 0 ? SEMANTIC_PROJECTION_EXTENT / maxAbs : 0;
  for (const [path, xy] of entries) {
    out.set(path, [xy[0] * scale, xy[1] * scale]);
  }
  return out;
}

export interface AdaptedSemanticGraph {
  nodes: GraphNodeInput[];
  links: GraphLinkInput[];
  palette: Map<string, string>;
}

/**
 * Map the API payload onto the engine's generic `{nodes, links}` contract,
 * conforming to `AdaptedEngineGraph` so the wiki graph workspace can render it.
 *
 * The visualization mode decides three things: the node `group` and colour
 * (kind, or the semantic cluster when `mode` clusters — named from the payload's
 * `clusterLabels`, never from the raw index), whether `x`/`y` carry the 2D
 * projection (`projection`/`combined`), and whether the neighbour edges are kept
 * (`edges`/`clusters`/`combined`; dropped in the pure `projection` map, which is
 * points, not links).
 */
export function adaptSemanticGraph(
  graph: SemanticGraph,
  mode: SemanticMode = "edges",
): AdaptedEngineGraph {
  const nodes = Array.isArray(graph?.nodes) ? graph.nodes : [];
  const edges = Array.isArray(graph?.edges) ? graph.edges : [];
  const byCluster = semanticModeClusters(mode);
  const projected = semanticModeProjects(mode) ? scaledProjection(graph?.projection) : null;

  const groupOf = (n: SemanticNode): string => {
    if (!byCluster) return n.kind || "unknown";
    const index = graph?.clusters?.[n.path];
    if (index === undefined) return "cluster ?";
    return graph?.clusterLabels?.[String(index)] ?? `cluster ${index}`;
  };

  const palette = clusterPalette(
    nodes.map(
      (n) => ({ id: n.path, title: n.title ?? n.path, type: groupOf(n) }) as unknown as GraphNode,
    ),
    "type",
  );

  const mapped: GraphNodeInput[] = nodes.map((n) => {
    const group = groupOf(n);
    const xy = projected?.get(n.path);
    return {
      id: n.path,
      label: n.title || n.path,
      group,
      color: palette.get(group) ?? DEFAULT_NODE_COLOR,
      description: n.summary,
      path: n.path,
      ...(xy ? { x: xy[0], y: xy[1] } : {}),
    };
  });

  const links: GraphLinkInput[] =
    mode === "projection"
      ? []
      : (() => {
          const ids = new Set(mapped.map((n) => n.id));
          const present = edges.filter((e) => ids.has(e.from) && ids.has(e.to));
          // A clustered view draws the strongest few: the rest is the density that
          // hides the grouping, and the edge map keeps all of them.
          const drawn = byCluster ? thinEdges(present, semanticEdgeBudget(mapped.length)) : present;
          return drawn.map((e) => ({
            source: e.from,
            target: e.to,
            type: "semantic",
            color: SEMANTIC_EDGE_COLOR,
            weight: e.score,
          }));
        })();

  return { nodes: mapped, links, palette, allVerbs: ["semantic"], allKinds: [] };
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
