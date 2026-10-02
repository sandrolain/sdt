import { polygonHull } from "d3-polygon";
import type { NodeRectLike } from "./boundaries";
import type { Group } from "./groups";

export interface GroupHull {
  id: string;
  path: string;
  labelX: number;
  labelY: number;
  color: string;
}

export const GROUP_PADDING = 8;

const PALETTE = [
  "#cba6f7",
  "#a6e3a1",
  "#f9e2af",
  "#74c7ec",
  "#f5c2e7",
  "#94e2d5",
  "#89b4fa",
  "#eba0ac",
];

/** Deterministic Catppuccin accent for a group id, independent of member order. */
export function groupColor(id: string): string {
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = (hash * 31 + id.charCodeAt(i)) >>> 0;
  }
  return PALETTE[hash % PALETTE.length];
}

type Point = [number, number];

function pathOf(points: Point[]): string {
  const [first, ...rest] = points;
  return `M${first[0]},${first[1]}${rest.map(([x, y]) => `L${x},${y}`).join("")}Z`;
}

function bounds(points: Point[]) {
  const xs = points.map((p) => p[0]);
  const ys = points.map((p) => p[1]);
  return {
    minX: Math.min(...xs),
    minY: Math.min(...ys),
    maxX: Math.max(...xs),
    maxY: Math.max(...ys),
  };
}

/**
 * Convex-hull shape per group: the member rects are inflated by `padding`, then
 * hulled, so scattered members are wrapped by one path. Groups of fewer than
 * three members (and any collinear set) fall back to a padded bounding rect;
 * groups whose members have no rect are skipped.
 */
export function groupHulls(
  groups: Group[],
  rects: Map<string, NodeRectLike>,
  padding = GROUP_PADDING,
): GroupHull[] {
  const out: GroupHull[] = [];
  for (const group of groups) {
    const boxes = group.nodes
      .map((n) => rects.get(n.id))
      .filter((r): r is NodeRectLike => Boolean(r));
    if (boxes.length === 0) continue;
    const corners: Point[] = [];
    for (const r of boxes) {
      const x0 = r.x - padding;
      const y0 = r.y - padding;
      const x1 = r.x + r.width + padding;
      const y1 = r.y + r.height + padding;
      corners.push([x0, y0], [x1, y0], [x1, y1], [x0, y1]);
    }
    const box = bounds(corners);
    const points =
      boxes.length >= 3
        ? (polygonHull(corners) ?? [
            [box.minX, box.minY],
            [box.maxX, box.minY],
            [box.maxX, box.maxY],
            [box.minX, box.maxY],
          ])
        : ([
            [box.minX, box.minY],
            [box.maxX, box.minY],
            [box.maxX, box.maxY],
            [box.minX, box.maxY],
          ] as Point[]);
    const hb = bounds(points);
    out.push({
      id: group.id,
      path: pathOf(points),
      labelX: hb.minX + 8,
      labelY: hb.minY + 15,
      color: groupColor(group.id),
    });
  }
  return out;
}
