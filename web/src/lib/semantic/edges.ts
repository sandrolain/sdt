/**
 * Edge budget for the clustered views of the docs map.
 *
 * The payload's edge set is already the de-duplicated top-k per node, which is
 * sparse in the edge map and still too dense to read once the nodes are grouped:
 * every long, weak neighbour edge crosses the picture and washes the grouping out
 * (analysis 20261013-063009, O5). A clustered mode therefore draws the strongest
 * few, and the rule is pure so the budget is a number a test can hold.
 */
import type { SemanticEdge } from "./graph";

/** The edges a clustered view draws: one per node, floored and capped, so a tiny
 * map keeps its shape and a large one stays readable. */
export function semanticEdgeBudget(nodeCount: number): number {
  return Math.max(24, Math.min(240, nodeCount));
}

/**
 * The strongest `max` edges, ordered by score descending and ties by their ends, so
 * the same payload always yields the same picture. A budget that already covers the
 * set returns it unchanged.
 */
export function thinEdges(edges: SemanticEdge[], max: number): SemanticEdge[] {
  if (edges.length <= max) return edges;
  const ranked = [...edges].sort(
    (a, b) => b.score - a.score || a.from.localeCompare(b.from) || a.to.localeCompare(b.to),
  );
  return ranked.slice(0, max);
}
