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

// Categorical identity, not a theme role (phase 9): a cluster id *is* an
// identity, so its colour is a palette entry and stays a raw value. The renderer
// draws it on a canvas, where the active theme's CSS tokens do not apply.
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

// Categorical identity (phase 9): an unclustered node is an identity in the
// graph palette, not a theme role; the renderer draws it on a canvas.
export const DEFAULT_NODE_COLOR = "#9399b2";

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

export function fetchWikiGraph(): Promise<GraphData> {
  return fetchJSON<GraphData>("/api/wiki/graph");
}
