/**
 * Side-anchored edge geometry for the shared JSON Canvas view.
 *
 * Adapted from `context/refs/react/jc-vite/src/JsonCanvas.jsx` (MIT). Pure: no
 * React and no DOM, so the 2D canvas, the layered 3D view and the SVG/PNG export
 * all consume the same `edgeGeom` result and cannot diverge.
 */
import type { CanvasEdge, CanvasNode, CanvasSide } from "./document";

/** Outward unit normal of each side. */
export const SIDES: Record<CanvasSide, readonly [number, number]> = {
  top: [0, -1],
  right: [1, 0],
  bottom: [0, 1],
  left: [-1, 0],
};

export interface Point {
  x: number;
  y: number;
}

/** Euclidean distance between two points. */
export function dist(a: Point, b: Point): number {
  return Math.hypot(a.x - b.x, a.y - b.y);
}

/** Point on a node's side (midpoint for top/bottom/left/right). */
export function anchor(n: Pick<CanvasNode, "x" | "y" | "width" | "height">, s: CanvasSide): Point {
  return {
    x: n.x + n.width * (s === "left" ? 0 : s === "right" ? 1 : 0.5),
    y: n.y + n.height * (s === "top" ? 0 : s === "bottom" ? 1 : 0.5),
  };
}

/** A node is inside a group when fully contained (and not the group itself). */
export function inside(g: CanvasNode, n: CanvasNode): boolean {
  return (
    n.id !== g.id &&
    n.x >= g.x &&
    n.y >= g.y &&
    n.x + n.width <= g.x + g.width &&
    n.y + n.height <= g.y + g.height
  );
}

/** Aspect-normalized side choice used when an edge names no side. */
export function autoSides(
  a: Pick<CanvasNode, "x" | "y" | "width" | "height">,
  b: Pick<CanvasNode, "x" | "y" | "width" | "height">,
): [CanvasSide, CanvasSide] {
  const dx = b.x + b.width / 2 - (a.x + a.width / 2);
  const dy = b.y + b.height / 2 - (a.y + a.height / 2);
  if (Math.abs(dx) / a.width > Math.abs(dy) / a.height) {
    return dx > 0 ? ["right", "left"] : ["left", "right"];
  }
  return dy > 0 ? ["bottom", "top"] : ["top", "bottom"];
}

export interface EdgeGeometry {
  p: Point;
  q: Point;
  c1: Point;
  c2: Point;
  da: readonly [number, number];
  db: readonly [number, number];
  /** Label anchor at the cubic Bézier t = 0.375 point. */
  mid: Point;
  /** SVG path `d`. */
  d: string;
}

/**
 * The cubic Bézier from `fromNode` to `toNode`: endpoints from explicit
 * `fromSide`/`toSide` when present, else `autoSides`; control points offset
 * along each side's outward normal by `max(40, dist × 0.4)`.
 */
export function edgeGeom(e: CanvasEdge, byId: Record<string, CanvasNode>): EdgeGeometry | null {
  const A = byId[e.fromNode];
  const B = byId[e.toNode];
  if (!A || !B) return null;
  const auto = autoSides(A, B);
  const sa = e.fromSide ?? auto[0];
  const sb = e.toSide ?? auto[1];
  const p = anchor(A, sa);
  const q = anchor(B, sb);
  const off = Math.max(40, dist(p, q) * 0.4);
  const da = SIDES[sa];
  const db = SIDES[sb];
  const c1 = { x: p.x + da[0] * off, y: p.y + da[1] * off };
  const c2 = { x: q.x + db[0] * off, y: q.y + db[1] * off };
  return {
    p,
    q,
    c1,
    c2,
    da,
    db,
    mid: {
      x: (p.x + 3 * c1.x + 3 * c2.x + q.x) / 8,
      y: (p.y + 3 * c1.y + 3 * c2.y + q.y) / 8,
    },
    d: `M${p.x},${p.y} C${c1.x},${c1.y} ${c2.x},${c2.y} ${q.x},${q.y}`,
  };
}

/** SVG `points` for an arrow head at `t` pointing along the outward normal `n`. */
export function arrow(t: Point, n: readonly [number, number], s = 12): string {
  return `${t.x},${t.y} ${t.x + n[0] * s - n[1] * s * 0.5},${t.y + n[1] * s + n[0] * s * 0.5} ${t.x + n[0] * s + n[1] * s * 0.5},${t.y + n[1] * s - n[0] * s * 0.5}`;
}
