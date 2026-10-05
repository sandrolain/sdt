/**
 * SDT → engine data adapter.
 *
 * Maps the read-only `/api/wiki/graph` payload (`GraphData`: nodes + typed
 * edges) onto the engine's generic `{nodes, links}` contract (analysis D3):
 * `verb` → link `type`, `kind` carried for filtering and the tooltip, cluster →
 * node `group`, title → `label`, summary → `description`. Filtering (groups,
 * relations, kinds) is applied by the engine's `setFilters`, which dims rather
 * than removes, so this keeps the model complete (B5/D4).
 */
import {
  clusterOf,
  clusterPalette,
  DEFAULT_NODE_COLOR,
  type ClusterKey,
  type GraphData,
} from "../graphModel";
import type { GraphLinkInput, GraphNodeInput } from "./types";

export interface AdaptedEngineGraph {
  nodes: GraphNodeInput[];
  links: GraphLinkInput[];
  palette: Map<string, string>;
  allVerbs: string[];
  allKinds: string[];
}

export interface AdapterOptions {
  clusterKey: ClusterKey;
}

export function adaptToEngine(data: GraphData, opts: AdapterOptions): AdaptedEngineGraph {
  const palette = clusterPalette(data.nodes, opts.clusterKey);
  const nodes: GraphNodeInput[] = data.nodes.map((n) => {
    const cluster = clusterOf(n, opts.clusterKey);
    return {
      id: n.id,
      label: n.title,
      group: cluster,
      color: palette.get(cluster) ?? DEFAULT_NODE_COLOR,
      description: n.summary,
      path: n.path,
      status: n.status,
      type: n.type,
      tags: n.tags,
    };
  });
  const links: GraphLinkInput[] = data.edges.map((e) => ({
    source: e.source,
    target: e.target,
    type: e.verb,
    label: e.label,
    kind: e.kind,
  }));
  const allVerbs = [...new Set(data.edges.map((e) => e.verb))].sort();
  const allKinds = [...new Set(data.edges.map((e) => e.kind))].sort();
  return { nodes, links, palette, allVerbs, allKinds };
}
