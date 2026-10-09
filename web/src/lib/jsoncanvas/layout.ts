/**
 * Dynamic canvas layout (S3.4): a pure, deterministic positioner. A node is
 * grouped by geometric containment (a non-group node fully inside a `group`
 * rect belongs to it); each group's children are packed in a grid inside it and
 * the group is resized to fit; ungrouped nodes are packed in a top-level grid
 * below the groups. Positions are computed in memory — the canvas file is never
 * written.
 */
import { nodeInside } from "./collapse";
import type { CanvasDocument, CanvasNode } from "./document";

export interface LayoutPosition {
  x: number;
  y: number;
  width?: number;
  height?: number;
}

export interface LayoutOptions {
  gap?: number;
  padding?: number;
}

const DEFAULT_GAP = 40;
const DEFAULT_PADDING = 48;

/** A near-square grid of cell positions, top-left first, row-major. */
function grid(
  children: CanvasNode[],
  originX: number,
  originY: number,
  gap: number,
): LayoutPosition[] {
  const cols = Math.max(1, Math.ceil(Math.sqrt(children.length)));
  const cells: LayoutPosition[] = [];
  let x = originX;
  let y = originY;
  let rowH = 0;
  children.forEach((c, i) => {
    if (i > 0 && i % cols === 0) {
      x = originX;
      y += rowH + gap;
      rowH = 0;
    }
    cells.push({ x, y });
    x += c.width + gap;
    rowH = Math.max(rowH, c.height);
  });
  return cells;
}

export function layoutCanvas(
  doc: CanvasDocument,
  opts: LayoutOptions = {},
): Map<string, LayoutPosition> {
  const gap = opts.gap ?? DEFAULT_GAP;
  const padding = opts.padding ?? DEFAULT_PADDING;
  const positions = new Map<string, LayoutPosition>();
  const groups = doc.nodes.filter((n) => n.type === "group");
  const contained = new Set<string>();

  for (const g of groups) {
    const children = doc.nodes.filter(
      (n) => n.type !== "group" && !contained.has(n.id) && nodeInside(g, n),
    );
    if (children.length === 0) continue;
    const cells = grid(children, g.x + padding, g.y + padding, gap);
    let maxX = g.x;
    let maxY = g.y;
    children.forEach((c, i) => {
      positions.set(c.id, cells[i]);
      contained.add(c.id);
      maxX = Math.max(maxX, cells[i].x + c.width);
      maxY = Math.max(maxY, cells[i].y + c.height);
    });
    positions.set(g.id, {
      x: g.x,
      y: g.y,
      width: maxX - g.x + padding,
      height: maxY - g.y + padding,
    });
  }

  const free = doc.nodes.filter((n) => n.type !== "group" && !contained.has(n.id));
  if (free.length > 0) {
    const below = groups.reduce((m, g) => {
      const p = positions.get(g.id);
      return Math.max(m, p ? p.y + (p.height ?? g.height) : g.y + g.height);
    }, 0);
    const cells = grid(free, 0, below + (groups.length ? gap : 0), gap);
    free.forEach((c, i) => positions.set(c.id, cells[i]));
  }

  return positions;
}
