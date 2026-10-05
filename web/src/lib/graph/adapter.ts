/**
 * SDT → engine data adapter.
 *
 * Maps the read-only `/api/wiki/graph` payload (`GraphData`: nodes + typed
 * edges) onto the engine's generic `{nodes, links}` contract (analysis D3):
 * `verb` → link `type`, `kind` carried for filtering and the tooltip, cluster →
 * node `group`, title → `label`, summary → `description`. SDT's verb/kind
 * filtering is applied here (pre-filter edges) rather than in the engine.
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
  /** verbs to keep; undefined keeps all */
  visibleVerbs?: Set<string>;
  /** edge kinds to keep; undefined keeps all */
  visibleKinds?: Set<string>;
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
  const links: GraphLinkInput[] = data.edges
    .filter((e) => (opts.visibleVerbs ? opts.visibleVerbs.has(e.verb) : true))
    .filter((e) => (opts.visibleKinds ? opts.visibleKinds.has(e.kind) : true))
    .map((e) => ({
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
