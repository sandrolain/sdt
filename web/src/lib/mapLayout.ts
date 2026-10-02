import { METRICS, flattenMeasured, type MeasuredNode } from "./mapMetrics";

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

/** Leaves of a subtree, in document order — the unit both layouts work in. */
function leavesOf(node: MeasuredNode): MeasuredNode[] {
  if (node.children.length === 0) return [node];
  return node.children.flatMap(leavesOf);
}

/** Vertical extent a subtree needs: its leaves plus the gaps between them. */
function subtreeHeight(node: MeasuredNode): number {
  if (node.children.length === 0) return node.box.height;
  const sum = node.children.reduce((acc, child) => acc + subtreeHeight(child), 0);
  return sum + METRICS.siblingGap * (node.children.length - 1);
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
  if (node.children.length === 0) {
    const y = top + node.box.height / 2;
    out.set(node.id, { x, y });
    return y;
  }
  let cursor = top;
  const childYs: number[] = [];
  for (const child of node.children) {
    childYs.push(stackY(child, xs, side, cursor, out));
    cursor += subtreeHeight(child) + METRICS.siblingGap;
  }
  const y = (childYs[0] + childYs[childYs.length - 1]) / 2;
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
 * Concentric layout: every leaf owns an equal angular slot in document order,
 * a parent sits at the angular mean of its leaves, and depth becomes the radius.
 *
 * Non-overlap is by construction rather than by luck: two points are at least
 * as far apart as the difference of their radii, so consecutive rings are
 * separated by more than the largest possible box diagonal, and each ring is
 * pushed out until the chord between its closest slots clears a box width.
 */
export function layoutRadial(root: MeasuredNode): Map<string, MapPoint> {
  const positions = new Map<string, MapPoint>();
  positions.set(root.id, { x: 0, y: 0 });
  if (root.children.length === 0) return positions;
  const nodes = flattenMeasured(root);
  const leaves = leavesOf(root);
  const maxDepth = Math.max(...nodes.map((n) => n.depth));
  const widest = Math.max(...nodes.map((n) => Math.max(n.box.width, n.box.height)));

  const ringGap = Math.ceil(Math.SQRT2 * widest) + METRICS.siblingGap;
  const radii: number[] = [0];
  for (let d = 1; d <= maxDepth; d++) radii[d] = radii[d - 1] + ringGap;

  // The chord between neighbouring slots must clear a box width, so a crowded
  // ring moves outward; the factor applies to every ring, keeping them apart.
  // A ring counts every node on it: parents share their leaves' angles and are
  // as crowded as the leaves are.
  let scale = 1;
  for (let d = 1; d <= maxDepth; d++) {
    const perRing = Math.max(nodes.filter((n) => n.depth === d).length, 2);
    const needed = (widest + METRICS.siblingGap) / (2 * Math.sin(Math.PI / perRing));
    scale = Math.max(scale, needed / radii[d]);
  }

  const step = (2 * Math.PI) / leaves.length;
  const angles = new Map<string, number>();
  // Start at the top and run clockwise, the way the balanced tree reads.
  leaves.forEach((leaf, i) => angles.set(leaf.id, -Math.PI / 2 + i * step));
  const place = (node: MeasuredNode): void => {
    for (const child of node.children) place(child);
    if (!angles.has(node.id)) {
      const own = leavesOf(node).map((l) => angles.get(l.id) ?? 0);
      angles.set(node.id, (own[0] + own[own.length - 1]) / 2);
    }
    const r = radii[node.depth] * scale;
    const angle = angles.get(node.id) ?? 0;
    positions.set(node.id, { x: r * Math.cos(angle), y: r * Math.sin(angle) });
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
