/**
 * Layout maths for the ported graph engine.
 *
 * The reference lays out `force`/`groups`/`radial` continuously in its tick
 * (`KnowledgeGraph.jsx` 780-846, 990-1007); this module keeps the pure pieces
 * (link parameters, group centres, radial rings) and adds SDT's deterministic
 * `hierarchy` (dagre) and `circular` positioners from the former `graphLayout.ts`. The
 * positioners back the engine's pre-positioned/relaxed mode (analysis A4).
 */
import dagre from "@dagrejs/dagre";
import type { GraphLayout } from "./types";

/** Sidebar options for the engine layouts, in display order. */
export const LAYOUT_OPTIONS: { id: GraphLayout; label: string }[] = [
  { id: "force", label: "Force" },
  { id: "groups", label: "Groups" },
  { id: "radial", label: "Radial" },
  { id: "hierarchy", label: "Hierarchy" },
  { id: "circular", label: "Circular" },
];

export interface Physics {
  charge: number;
  linkDistance: number;
  gravity: number;
  maxDist: number;
  alphaMin: number;
  alphaDecay: number;
  ringGap: number;
}

/** Reference defaults (`KnowledgeGraph.jsx` 240). */
export const DEFAULT_PHYSICS: Physics = {
  charge: 48,
  linkDistance: 38,
  gravity: 0.045,
  maxDist: 520,
  alphaMin: 0.003,
  alphaDecay: 0.978,
  ringGap: 64,
};

export interface LayoutNodeLike {
  id: string;
  /** cluster / group key */
  group: string;
  /** degree (link count) */
  deg: number;
  index: number;
}

export interface LayoutLinkLike {
  /** source node index */
  a: number;
  /** target node index */
  b: number;
  type?: string;
  weight?: number;
}

/** Repulsion strength, spring length and directional bias per link (reference 793-807). */
export function computeLinkParams(
  layout: string,
  nodes: LayoutNodeLike[],
  links: LayoutLinkLike[],
  groupIndex: Int32Array,
  linkDistance: number,
): { lStr: Float32Array; lDist: Float32Array; lBias: Float32Array } {
  const m = links.length;
  const lStr = new Float32Array(m);
  const lDist = new Float32Array(m);
  const lBias = new Float32Array(m);
  links.forEach((l, li) => {
    const da = nodes[l.a].deg;
    const db = nodes[l.b].deg;
    let s = 1 / Math.max(1, Math.min(da, db));
    let d = linkDistance;
    if (layout === "groups" && groupIndex[l.a] !== groupIndex[l.b]) {
      s *= 0.18;
      d *= 2.2;
    }
    if (layout === "radial") s *= 0.35;
    const w = l.weight;
    if (typeof w === "number") s *= Math.min(2, Math.max(0.2, w));
    lStr[li] = s;
    lDist[li] = d;
    lBias[li] = da / Math.max(1, da + db);
  });
  return { lStr, lDist, lBias };
}

/** Golden-angle group centres that flatten onto a circle as the morph reaches 2D (reference 831-846). */
export function groupCenters(groupCount: number, n: number, flat: number): Float32Array {
  const c = new Float32Array(groupCount * 3);
  const R = 50 + 8 * Math.sqrt(n);
  for (let k = 0; k < groupCount; k += 1) {
    if (groupCount === 1) {
      c[k * 3] = 0;
      c[k * 3 + 1] = 0;
      c[k * 3 + 2] = 0;
      continue;
    }
    const y3 = 1 - (2 * (k + 0.5)) / groupCount;
    const rr = Math.sqrt(Math.max(0, 1 - y3 * y3));
    const th = k * 2.399963;
    const x3 = Math.cos(th) * rr * R;
    const yy3 = y3 * R;
    const z3 = Math.sin(th) * rr * R;
    const ang = (2 * Math.PI * k) / groupCount;
    const x2 = Math.cos(ang) * R * 1.1;
    const y2 = Math.sin(ang) * R * 1.1;
    c[k * 3] = x3 + (x2 - x3) * flat;
    c[k * 3 + 1] = yy3 + (y2 - yy3) * flat;
    c[k * 3 + 2] = z3 * (1 - flat);
  }
  return c;
}

/** BFS depth rings for the radial layout (reference 809-828); null when empty. */
export function radialRings(
  adj: { o: number }[][],
  n: number,
  sel: number,
  order: number[],
  ringGap: number,
): Float32Array | null {
  if (!n) return null;
  const root = sel >= 0 ? sel : (order[0] ?? 0);
  const depth = new Int32Array(n).fill(-1);
  const queue = [root];
  depth[root] = 0;
  for (let qi = 0; qi < queue.length; qi += 1) {
    const u = queue[qi];
    for (const e of adj[u]) {
      if (depth[e.o] < 0) {
        depth[e.o] = depth[u] + 1;
        queue.push(e.o);
      }
    }
  }
  let md = 0;
  for (let i = 0; i < n; i += 1) md = Math.max(md, depth[i]);
  for (let i = 0; i < n; i += 1) if (depth[i] < 0) depth[i] = md + 1;
  const count = new Int32Array(md + 2);
  for (let i = 0; i < n; i += 1) count[depth[i]] += 1;
  const R = new Float32Array(md + 2);
  for (let d = 1; d < R.length; d += 1) {
    R[d] = Math.max(R[d - 1] + ringGap, (count[d] * 24) / (2 * Math.PI));
  }
  const out = new Float32Array(n);
  for (let i = 0; i < n; i += 1) out[i] = R[depth[i]];
  return out;
}

/** Verbs rendered as hierarchy edges, parent → child (former graphLayout.ts 14). */
const HIERARCHY_VERBS = new Set(["part_of", "contains", "depends_on"]);

/** Dagre top-down positions over the hierarchy verbs; index-aligned n*3 array. */
export function hierarchyPositions(nodes: LayoutNodeLike[], links: LayoutLinkLike[]): Float32Array {
  const g = new dagre.graphlib.Graph({ multigraph: true });
  g.setGraph({ rankdir: "TB", nodesep: 40, ranksep: 90 });
  g.setDefaultEdgeLabel(() => ({}));
  for (const n of nodes) g.setNode(n.id, { width: 60, height: 24 });
  for (const l of links) {
    if (!HIERARCHY_VERBS.has(l.type ?? "")) continue;
    const s = nodes[l.a]?.id;
    const t = nodes[l.b]?.id;
    if (!s || !t || !g.hasNode(s) || !g.hasNode(t)) continue;
    if (l.type === "part_of" || l.type === "depends_on") g.setEdge(t, s, {}, l.type);
    else g.setEdge(s, t, {}, l.type);
  }
  dagre.layout(g);
  const pos = new Float32Array(nodes.length * 3);
  for (const n of nodes) {
    const p = g.node(n.id);
    pos[n.index * 3] = (p?.x ?? 0) * 1.6;
    pos[n.index * 3 + 1] = (p?.y ?? 0) * 1.6;
    pos[n.index * 3 + 2] = 0;
  }
  return pos;
}

/** Deterministic circular positions grouped by cluster (former `graphLayout.ts` 108-120). */
export function circularPositions(nodes: LayoutNodeLike[]): Float32Array {
  const pos = new Float32Array(nodes.length * 3);
  const groups = new Map<string, LayoutNodeLike[]>();
  for (const n of [...nodes].sort(
    (a, b) => a.group.localeCompare(b.group) || a.id.localeCompare(b.id),
  )) {
    const list = groups.get(n.group) ?? [];
    list.push(n);
    groups.set(n.group, list);
  }
  const radius = 220;
  let gi = 0;
  for (const group of groups.values()) {
    group.forEach((entry, i) => {
      const angle = (i / Math.max(group.length, 1)) * Math.PI * 2 + gi * 0.3;
      pos[entry.index * 3] = Math.cos(angle) * radius;
      pos[entry.index * 3 + 1] = Math.sin(angle) * radius;
      pos[entry.index * 3 + 2] = gi * 40;
    });
    gi += 1;
  }
  return pos;
}
