import { METRICS, flattenMeasured, type MeasuredNode } from "./mapMetrics";
import type { CanvasSide } from "./jsoncanvas/document";

/**
 * Map layouts (plan D9): two deterministic position functions over a measured
 * tree, no layout dependency. `balanced` is a tidy two-sided tree — the root's
 * children are split left and right by subtree size, siblings stack, parents
 * centre on their children — and `radial` places each depth on its own ring,
 * every node at the angular mean of its leaves. Both guarantee by construction
 * that no two boxes overlap.
 */

export type MapLayoutKind = "balanced" | "radial";

export const MAP_LAYOUTS: { id: MapLayoutKind; label: string }[] = [
  { id: "balanced", label: "Balanced" },
  { id: "radial", label: "Radial" },
];

export interface MapPoint {
  x: number;
  y: number;
}

export interface MapBounds {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
  width: number;
  height: number;
}

export interface LayoutResult {
  positions: Map<string, MapPoint>;
  bounds: MapBounds;
}

/**
 * Outward side a parent→child edge leaves/enters, from the layout (behaviour
 * decision 1): balanced never leaves horizontally-excepted sides — it is a
 * column layout, so a parent edge is always right/left even when the child is
 * far vertically; radial picks the dominant axis of the parent→child vector.
 * Relation edges (`r:`) keep the component fallback and are not passed here.
 */
export function edgeSides(
  kind: MapLayoutKind,
  parent: MapPoint,
  child: MapPoint,
): [CanvasSide, CanvasSide] {
  if (kind === "balanced") {
    return child.x >= parent.x ? ["right", "left"] : ["left", "right"];
  }
  const dx = child.x - parent.x;
  const dy = child.y - parent.y;
  if (Math.abs(dx) >= Math.abs(dy)) return dx >= 0 ? ["right", "left"] : ["left", "right"];
  return dy >= 0 ? ["bottom", "top"] : ["top", "bottom"];
}

/** Leaves of a subtree, in document order — the unit both layouts work in. */
function leavesOf(node: MeasuredNode): MeasuredNode[] {
  if (node.children.length === 0) return [node];
  return node.children.flatMap(leavesOf);
}

/**
 * Vertical extent a subtree needs: its own box (an internal node may be taller
 * than the stack its leaves reserve, e.g. a 5-line label) or its leaves plus the
 * gaps between them, whichever is larger. This is the slot the sibling cursor
 * advances by, so a tall internal node can no longer overlap a same-column
 * sibling (analysis defect 3).
 */
function subtreeHeight(node: MeasuredNode): number {
  if (node.children.length === 0) return node.box.height;
  const stack =
    node.children.reduce((acc, child) => acc + subtreeHeight(child), 0) +
    METRICS.siblingGap * (node.children.length - 1);
  return Math.max(node.box.height, stack);
}

/** Width of each depth column, so columns never overlap. */
function columnWidths(nodes: MeasuredNode[]): number[] {
  const widths: number[] = [];
  for (const node of nodes) {
    widths[node.depth] = Math.max(widths[node.depth] ?? 0, node.box.width);
  }
  return widths;
}

/** Signed x of a depth column: depth 1 is the first column off the root. */
function columnXs(widths: number[]): number[] {
  const xs: number[] = [0];
  for (let d = 1; d < widths.length; d++) {
    xs[d] = xs[d - 1] + (widths[d - 1] ?? 0) + METRICS.depthGap;
  }
  return xs;
}

/** Assign each node a y: leaves stack from `top`, parents centre on children. */
function stackY(
  node: MeasuredNode,
  xs: number[],
  side: 1 | -1,
  top: number,
  out: Map<string, MapPoint>,
): number {
  const x = side * (xs[node.depth] ?? 0);
  const slot = subtreeHeight(node);
  if (node.children.length === 0) {
    const y = top + node.box.height / 2;
    out.set(node.id, { x, y });
    return y;
  }
  // Centre the children stack inside the node's slot, so a node taller than its
  // leaves reserves the extra room symmetrically and cannot spill into a sibling.
  const childStack =
    node.children.reduce((acc, child) => acc + subtreeHeight(child), 0) +
    METRICS.siblingGap * (node.children.length - 1);
  let cursor = top + (slot - childStack) / 2;
  for (const child of node.children) {
    stackY(child, xs, side, cursor, out);
    cursor += subtreeHeight(child) + METRICS.siblingGap;
  }
  const y = top + slot / 2;
  out.set(node.id, { x, y });
  return y;
}

/** Split the root's children between the two sides, keeping document order. */
function splitSides(root: MeasuredNode): { left: MeasuredNode[]; right: MeasuredNode[] } {
  const left: MeasuredNode[] = [];
  const right: MeasuredNode[] = [];
  let leftWeight = 0;
  let rightWeight = 0;
  for (const child of root.children) {
    const weight = leavesOf(child).length;
    if (leftWeight <= rightWeight) {
      left.push(child);
      leftWeight += weight;
    } else {
      right.push(child);
      rightWeight += weight;
    }
  }
  return { left, right };
}

/** Tidy two-sided layout: root centred, subtrees growing left and right. */
export function layoutBalanced(root: MeasuredNode): Map<string, MapPoint> {
  const positions = new Map<string, MapPoint>();
  positions.set(root.id, { x: 0, y: 0 });
  const { left, right } = splitSides(root);
  const xs = columnXs(columnWidths(flattenMeasured(root)));
  for (const [side, nodes] of [
    [1, right],
    [-1, left],
  ] as [1 | -1, MeasuredNode[]][]) {
    if (nodes.length === 0) continue;
    // Each side is laid out on its own, then centred on the root's row.
    const heights = nodes.map((n) => subtreeHeight(n));
    const total = heights.reduce((a, b) => a + b, 0) + METRICS.siblingGap * (nodes.length - 1);
    const sidePositions = new Map<string, MapPoint>();
    let cursor = -total / 2;
    for (const child of nodes) {
      stackY(child, xs, side, cursor, sidePositions);
      cursor += subtreeHeight(child) + METRICS.siblingGap;
    }
    for (const [id, point] of sidePositions) positions.set(id, point);
  }
  return positions;
}

/**
 * Concentric layout: every leaf owns an equal angular slot in document order, an
 * internal node owns the arc spanning its leaves and sits at the arc centre, and
 * depth becomes the radius.
 *
 * Non-overlap is by construction: sibling subtrees have **disjoint** angular
 * spans, and each ring radius is chosen so every node's box fits inside its own
 * span — `r ≥ D / sin(halfSpan)` with `D` the box half-diagonal — so two boxes in
 * disjoint spans cannot share an interior. Rings stay separated by at least
 * `ringGap` and monotonic with depth.
 */
export function layoutRadial(root: MeasuredNode): Map<string, MapPoint> {
  const positions = new Map<string, MapPoint>();
  positions.set(root.id, { x: 0, y: 0 });
  if (root.children.length === 0) return positions;

  const nodes = flattenMeasured(root);
  const leaves = leavesOf(root);
  const maxDepth = Math.max(...nodes.map((n) => n.depth));
  const widest = Math.max(...nodes.map((n) => Math.max(n.box.width, n.box.height)));

  // The leaf-index range of each subtree, assigned in document order.
  const range = new Map<string, [number, number]>();
  let leafIndex = 0;
  const walk = (node: MeasuredNode): [number, number] => {
    if (node.children.length === 0) {
      const i = leafIndex++;
      range.set(node.id, [i, i]);
      return [i, i];
    }
    let lo = Infinity;
    let hi = -Infinity;
    for (const child of node.children) {
      const [a, b] = walk(child);
      lo = Math.min(lo, a);
      hi = Math.max(hi, b);
    }
    range.set(node.id, [lo, hi]);
    return [lo, hi];
  };
  for (const child of root.children) walk(child);

  const step = (2 * Math.PI) / leaves.length;
  const angleOf = (nodeId: string): number => {
    const [lo, hi] = range.get(nodeId) ?? [0, 0];
    return -Math.PI / 2 + ((lo + hi) / 2) * step;
  };
  const halfSpanOf = (nodeId: string): number => {
    const [lo, hi] = range.get(nodeId) ?? [0, 0];
    return ((hi - lo + 1) * step) / 2;
  };

  // Each ring is at least ringGap beyond the previous, and far enough that every
  // box at that depth fits its angular span (half-diagonal over sin(half-span)).
  const ringGap = Math.ceil(Math.SQRT2 * widest) + METRICS.siblingGap;
  const radii: number[] = [0];
  for (let d = 1; d <= maxDepth; d++) {
    let needed = radii[d - 1] + ringGap;
    for (const node of nodes) {
      if (node.depth !== d) continue;
      const half = halfSpanOf(node.id);
      const halfDiagonal = Math.hypot(node.box.width, node.box.height) / 2;
      const s = Math.sin(Math.min(half, Math.PI / 2));
      needed = Math.max(needed, s > 0 ? halfDiagonal / s : halfDiagonal);
    }
    radii[d] = needed;
  }

  const place = (node: MeasuredNode): void => {
    const angle = angleOf(node.id);
    const r = radii[node.depth] ?? 0;
    positions.set(node.id, { x: r * Math.cos(angle), y: r * Math.sin(angle) });
    for (const child of node.children) place(child);
  };
  for (const child of root.children) place(child);
  return positions;
}

/** The bounding box of a layout, node boxes included. */
export function layoutBounds(root: MeasuredNode, positions: Map<string, MapPoint>): MapBounds {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const node of flattenMeasured(root)) {
    const point = positions.get(node.id);
    if (!point) continue;
    minX = Math.min(minX, point.x - node.box.width / 2);
    minY = Math.min(minY, point.y - node.box.height / 2);
    maxX = Math.max(maxX, point.x + node.box.width / 2);
    maxY = Math.max(maxY, point.y + node.box.height / 2);
  }
  if (!Number.isFinite(minX)) return { minX: 0, minY: 0, maxX: 0, maxY: 0, width: 0, height: 0 };
  return { minX, minY, maxX, maxY, width: maxX - minX, height: maxY - minY };
}

/** Position a measured tree with the named layout. */
export function layoutMap(root: MeasuredNode, kind: MapLayoutKind): LayoutResult {
  const positions = kind === "radial" ? layoutRadial(root) : layoutBalanced(root);
  return { positions, bounds: layoutBounds(root, positions) };
}
