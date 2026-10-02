import type { MeasuredNode } from "./mapMetrics";

/**
 * `#group/<name>` memberships: a cross-cutting tag inside a node's text, drawn
 * as one convex hull around every member wherever it sits — including members
 * that are not consecutive siblings, which an XMindMark `[B]` boundary cannot
 * wrap. The syntax is SDT's own convention, documented in
 * `context/instructions/mindmap-markmap.md`.
 */

export interface Group {
  id: string;
  nodes: MeasuredNode[];
}

/**
 * Collect groups from a measured tree: every node whose markers carry a
 * membership joins, regardless of position, so members may be non-consecutive.
 * Groups are returned in first-seen order.
 */
export function annotateGroups(root: MeasuredNode): Group[] {
  const order: string[] = [];
  const groups = new Map<string, Group>();
  const visit = (node: MeasuredNode) => {
    for (const id of node.markers.groups) {
      let group = groups.get(id);
      if (!group) {
        group = { id, nodes: [] };
        groups.set(id, group);
        order.push(id);
      }
      group.nodes.push(node);
    }
    node.children.forEach(visit);
  };
  visit(root);
  return order.map((id) => groups.get(id)!);
}
