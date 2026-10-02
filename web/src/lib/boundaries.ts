import type { MeasuredNode } from "./mapMetrics";
import type { MarkerTitles } from "./mapMarkers";

/**
 * XMindMark wrap groups and their geometry. A boundary (`[B]`) or a summary
 * (`[S]`) wraps **consecutive siblings** that carry the same marker id — one
 * group level, per the XMindMark specification. Membership comes from each
 * node's own annotations, so a duplicated topic text can no longer make two
 * nodes share a membership.
 *
 * The rectangles come from the computed layout (`mapModel.nodeRects`), not from
 * a renderer: the same geometry feeds the overlay and the exporter.
 */

export interface BoundaryGroup {
  id: string;
  title?: string;
  nodes: MeasuredNode[];
}

export interface BoundaryRect {
  id: string;
  title?: string;
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface NodeRectLike {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** Text content of a node, with HTML tags stripped. */
export function nodeText(node: { content: string }): string {
  return node.content
    .replace(/<[^>]*>/g, "")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .trim();
}

/** Walk sibling runs sharing the id their markers declare, under `titles`. */
function annotateWraps(
  root: MeasuredNode,
  slot: "boundary" | "summary",
  titles?: MarkerTitles,
): BoundaryGroup[] {
  const groups: BoundaryGroup[] = [];
  const declared = slot === "boundary" ? titles?.boundaries : titles?.summaries;
  const visit = (node: MeasuredNode) => {
    let current: BoundaryGroup | null = null;
    let currentId: string | null = null;
    for (const child of node.children) {
      const id = child.markers[slot];
      if (id !== undefined && id === currentId) {
        current?.nodes.push(child);
      } else if (id !== undefined) {
        current = { id, title: declared?.get(id), nodes: [child] };
        groups.push(current);
        currentId = id;
      } else {
        current = null;
        currentId = null;
      }
      visit(child);
    }
  };
  visit(root);
  return groups;
}

/** Consecutive same-boundary siblings, titled by their `[B<n>]: title` line. */
export function annotateBoundaries(root: MeasuredNode, titles?: MarkerTitles): BoundaryGroup[] {
  return annotateWraps(root, "boundary", titles);
}

/** Consecutive same-summary siblings, titled by their `[S<n>]: title` line. */
export function annotateSummaries(root: MeasuredNode, titles?: MarkerTitles): BoundaryGroup[] {
  return annotateWraps(root, "summary", titles);
}

/** Union rects of a group, expanded by padding; members without a rect are skipped. */
export function boundaryRects(
  groups: BoundaryGroup[],
  rects: Map<string, NodeRectLike>,
  padding = 8,
): BoundaryRect[] {
  const out: BoundaryRect[] = [];
  for (const group of groups) {
    const boxes = group.nodes
      .map((n) => rects.get(n.id))
      .filter((r): r is NodeRectLike => Boolean(r));
    if (boxes.length === 0) continue;
    const x = Math.min(...boxes.map((b) => b.x)) - padding;
    const y = Math.min(...boxes.map((b) => b.y)) - padding;
    const right = Math.max(...boxes.map((b) => b.x + b.width)) + padding;
    const bottom = Math.max(...boxes.map((b) => b.y + b.height)) + padding;
    out.push({ id: group.id, title: group.title, x, y, width: right - x, height: bottom - y });
  }
  return out;
}
