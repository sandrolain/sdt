/**
 * Group collapse (O6, `x-collapsed`): a `group` node marked collapsed hides the
 * non-group nodes fully contained in its rectangle, in 2D and 3D. Membership is
 * geometric (containment), so the emitter only needs enclosing group rects.
 */
import type { CanvasNode } from "./document";

/** A non-group node fully inside `outer`'s rectangle. */
export function nodeInside(outer: CanvasNode, node: CanvasNode): boolean {
  return (
    node.id !== outer.id &&
    node.x >= outer.x &&
    node.y >= outer.y &&
    node.x + node.width <= outer.x + outer.width &&
    node.y + node.height <= outer.y + outer.height
  );
}

/** Ids of the non-group nodes contained in any collapsed group. */
export function collapsedContainedIds(
  nodes: CanvasNode[],
  collapsed: ReadonlySet<string>,
): Set<string> {
  const hidden = new Set<string>();
  for (const g of nodes) {
    if (g.type !== "group" || !collapsed.has(g.id)) continue;
    for (const n of nodes) {
      if (n.type !== "group" && nodeInside(g, n)) hidden.add(n.id);
    }
  }
  return hidden;
}

/** Groups of a document with their authored collapse state (default false). */
export function boardGroups(
  nodes: CanvasNode[],
): { id: string; label: string; collapsed: boolean }[] {
  return nodes
    .filter((n) => n.type === "group")
    .map((n) => ({
      id: n.id,
      label: String(n.label ?? n.id),
      collapsed: n["x-collapsed"] === true,
    }));
}
