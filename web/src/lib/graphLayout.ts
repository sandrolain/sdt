import dagre from "@dagrejs/dagre";
import type { GLink, GNode } from "./graphModel";

export type LayoutKind = "force" | "radial" | "hierarchy" | "circular";

export const LAYOUTS: { id: LayoutKind; label: string }[] = [
  { id: "force", label: "Force" },
  { id: "radial", label: "Radial" },
  { id: "hierarchy", label: "Hierarchy" },
  { id: "circular", label: "Circular" },
];

/** Verbs rendered as hierarchy edges, parent → child. */
const HIERARCHY_VERBS = new Set(["part_of", "contains", "depends_on"]);

/** Deterministic PRNG (mulberry32) so force layouts seed identically. */
export function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

interface LayoutOptions {
  seed?: number;
  clusterKey?: string;
}

/**
 * Position nodes for the active layout. Force uses a deterministic seeded
 * spread; radial/circular group by cluster; hierarchy runs dagre over the
 * part_of/contains/depends_on edges. Node identity is preserved.
 */
export function applyLayout(
  nodes: GNode[],
  links: GLink[],
  kind: LayoutKind,
  opts: LayoutOptions = {},
): GNode[] {
  const next = nodes.map((n) => ({ ...n }));
  switch (kind) {
    case "force":
      return seedForce(next, opts.seed ?? 1);
    case "circular":
      return circular(next);
    case "radial":
      return radial(next);
    case "hierarchy":
      return hierarchy(next, links);
  }
}

function seedForce(nodes: GNode[], seed: number): GNode[] {
  const rng = mulberry32(seed);
  for (const n of nodes) {
    const angle = rng() * Math.PI * 2;
    const radius = 80 + rng() * 160;
    n.x = Math.cos(angle) * radius;
    n.y = Math.sin(angle) * radius;
    n.z = (rng() - 0.5) * 80;
    n.fx = undefined;
    n.fy = undefined;
  }
  return nodes;
}

function groupByCluster(nodes: GNode[]): Map<string, GNode[]> {
  const groups = new Map<string, GNode[]>();
  for (const n of [...nodes].sort(
    (a, b) => a.cluster.localeCompare(b.cluster) || a.id.localeCompare(b.id),
  )) {
    const list = groups.get(n.cluster) ?? [];
    list.push(n);
    groups.set(n.cluster, list);
  }
  return groups;
}

function circular(nodes: GNode[]): GNode[] {
  const groups = [...groupByCluster(nodes).values()];
  const radius = 220;
  groups.forEach((group, gi) => {
    group.forEach((n, i) => {
      const angle = (i / Math.max(group.length, 1)) * Math.PI * 2 + gi * 0.3;
      n.x = Math.cos(angle) * radius;
      n.y = Math.sin(angle) * radius;
      n.z = gi * 40;
    });
  });
  return nodes;
}

function radial(nodes: GNode[]): GNode[] {
  const groups = groupByCluster(nodes);
  let ring = 1;
  groups.forEach((group) => {
    const radius = 110 * ring;
    group.forEach((n, i) => {
      const angle = (i / Math.max(group.length, 1)) * Math.PI * 2;
      n.x = Math.cos(angle) * radius;
      n.y = Math.sin(angle) * radius;
      n.z = (ring - 1) * 60;
    });
    ring += 1;
  });
  return nodes;
}

/** Dagre top-down layout over hierarchy edges; other nodes join by height. */
function hierarchy(nodes: GNode[], links: GLink[]): GNode[] {
  const g = new dagre.graphlib.Graph({ multigraph: true });
  g.setGraph({ rankdir: "TB", nodesep: 40, ranksep: 90 });
  g.setDefaultEdgeLabel(() => ({}));
  for (const n of nodes) g.setNode(n.id, { width: 60, height: 24 });
  for (const l of links) {
    if (!HIERARCHY_VERBS.has(l.verb)) continue;
    const s = endpointId(l.source);
    const t = endpointId(l.target);
    if (!s || !t || !g.hasNode(s) || !g.hasNode(t)) continue;
    if (l.verb === "part_of") g.setEdge(t, s, {}, l.verb);
    else if (l.verb === "depends_on") g.setEdge(t, s, {}, l.verb);
    else g.setEdge(s, t, {}, l.verb);
  }
  dagre.layout(g);
  for (const n of nodes) {
    const pos = g.node(n.id);
    n.x = (pos?.x ?? 0) * 1.6;
    n.y = (pos?.y ?? 0) * 1.6;
    n.z = 0;
  }
  return nodes;
}

function endpointId(endpoint: unknown): string | null {
  if (typeof endpoint === "string") return endpoint;
  if (typeof endpoint === "object" && endpoint !== null && "id" in endpoint) {
    return String((endpoint as { id: unknown }).id);
  }
  return null;
}
