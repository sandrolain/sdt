import type { LinkObject, NodeObject } from "react-force-graph-2d";
import { fetchJSON } from "./fetchJson";

export interface GraphNode {
  id: string;
  title: string;
  type?: string;
  status?: string;
  tags?: string[];
  summary?: string;
  path: string;
}

export interface GraphEdge {
  source: string;
  target: string;
  verb: string;
  label?: string;
  kind: string;
}

export interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export type ClusterKey = "type" | "tag-root" | "status";

export const CLUSTER_KEYS: { id: ClusterKey; label: string }[] = [
  { id: "type", label: "Type" },
  { id: "tag-root", label: "Tag root" },
  { id: "status", label: "Status" },
];

/** Catppuccin Mocha accents used for canvas-rendered node colors. */
const CLUSTER_COLORS = [
  "#cba6f7",
  "#89b4fa",
  "#a6e3a1",
  "#fab387",
  "#94e2d5",
  "#f9e2af",
  "#f5c2e7",
  "#89dceb",
  "#b4befe",
  "#f38ba8",
  "#74c7ec",
  "#eba0ac",
];

export const DEFAULT_NODE_COLOR = "#9399b2";
export const DIM_NODE_COLOR = "#45475a";

/** Cluster id for a node under the active cluster key. */
export function clusterOf(node: GraphNode, key: ClusterKey): string {
  if (key === "status") return node.status?.trim() || "unknown";
  if (key === "tag-root") {
    const tag = node.tags?.[0];
    return tag ? tag.split("/")[0] : "untagged";
  }
  return node.type?.trim() || "unknown";
}

/** Stable cluster → color assignment (sorted cluster ids). */
export function clusterPalette(nodes: GraphNode[], key: ClusterKey): Map<string, string> {
  const ids = new Set<string>();
  for (const n of nodes) ids.add(clusterOf(n, key));
  const sorted = [...ids].sort();
  const palette = new Map<string, string>();
  sorted.forEach((id, i) => palette.set(id, CLUSTER_COLORS[i % CLUSTER_COLORS.length]));
  return palette;
}

export type GNode = NodeObject<GraphNode> & { cluster: string; color: string };
export type GLink = LinkObject<GraphNode, GraphEdge> & { color: string };

export interface GraphAdapterOptions {
  clusterKey: ClusterKey;
  /** verbs to keep; undefined keeps all */
  visibleVerbs?: Set<string>;
  /** edge kinds to keep; undefined keeps all */
  visibleKinds?: Set<string>;
}

export interface AdaptedGraph {
  nodes: GNode[];
  links: GLink[];
  palette: Map<string, string>;
  allVerbs: string[];
  allKinds: string[];
}

/** Stable key for a link, used by the highlight reducer. */
export function linkKey(link: { source: unknown; target: unknown; verb: string }): string {
  const s =
    typeof link.source === "object" && link.source !== null
      ? (link.source as GraphNode).id
      : String(link.source);
  const t =
    typeof link.target === "object" && link.target !== null
      ? (link.target as GraphNode).id
      : String(link.target);
  return `${s}--${link.verb}-->${t}`;
}

/** Derive node/link data for the force-graph renderers from the wiki API. */
export function adaptGraph(data: GraphData, opts: GraphAdapterOptions): AdaptedGraph {
  const degree = new Map<string, number>();
  for (const e of data.edges) {
    degree.set(e.source, (degree.get(e.source) ?? 0) + 1);
    degree.set(e.target, (degree.get(e.target) ?? 0) + 1);
  }
  const palette = clusterPalette(data.nodes, opts.clusterKey);
  const nodes: GNode[] = data.nodes.map((n) => {
    const cluster = clusterOf(n, opts.clusterKey);
    return {
      ...n,
      cluster,
      color: palette.get(cluster) ?? DEFAULT_NODE_COLOR,
      val: 1 + (degree.get(n.id) ?? 0),
    };
  });
  const links: GLink[] = data.edges
    .filter((e) => (opts.visibleVerbs ? opts.visibleVerbs.has(e.verb) : true))
    .filter((e) => (opts.visibleKinds ? opts.visibleKinds.has(e.kind) : true))
    .map((e) => ({
      ...e,
      color: "#6c7086",
    }));
  const allVerbs = [...new Set(data.edges.map((e) => e.verb))].sort();
  const allKinds = [...new Set(data.edges.map((e) => e.kind))].sort();
  return { nodes, links, palette, allVerbs, allKinds };
}

/** Undirected adjacency map from the adapted links. */
export function adjacency(links: GLink[]): Map<string, Set<string>> {
  const map = new Map<string, Set<string>>();
  const add = (a: string, b: string) => {
    if (!map.has(a)) map.set(a, new Set());
    map.get(a)!.add(b);
  };
  for (const l of links) {
    const s = idOf(l.source);
    const t = idOf(l.target);
    if (!s || !t) continue;
    add(s, t);
    add(t, s);
  }
  return map;
}

function idOf(endpoint: unknown): string | null {
  if (typeof endpoint === "string") return endpoint;
  if (typeof endpoint === "object" && endpoint !== null && "id" in endpoint) {
    return String((endpoint as { id: unknown }).id);
  }
  return null;
}

export function fetchWikiGraph(): Promise<GraphData> {
  return fetchJSON<GraphData>("/api/wiki/graph");
}
