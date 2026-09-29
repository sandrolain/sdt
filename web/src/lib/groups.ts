import { boundaryText, nodeText } from "./boundaries";
import type { MindNode } from "./mindmap";

export interface Group {
  id: string;
  nodes: MindNode[];
}

const GROUP_TAG_RE = /#group\/([A-Za-z0-9][\w-]*)/g;

/** Plain node text with any `#group/<name>` tags removed. */
export function stripGroupTags(content: string): string {
  return content
    .replace(GROUP_TAG_RE, "")
    .replace(/[ \t]{2,}/g, " ")
    .replace(/^\s+/, "")
    .replace(/\s+$/, "");
}

/**
 * Parse `#group/<name>` tags from page markdown, keyed by the plain node text
 * (`boundaryText`), mirroring `parseBoundaries`. A node may carry several tags;
 * the tag is node-scoped and independent of the document `tags:` frontmatter.
 * Nodes repeating the same plain text share the membership (same known
 * collision as `boundaries.ts`).
 */
export function parseGroups(md: string): Map<string, string[]> {
  const map = new Map<string, string[]>();
  for (const raw of md.split("\n")) {
    const ids = [...raw.matchAll(GROUP_TAG_RE)].map((m) => m[1]);
    if (ids.length === 0) continue;
    const text = stripGroupTags(boundaryText(raw));
    if (!text) continue;
    const existing = map.get(text) ?? [];
    for (const id of ids) {
      if (!existing.includes(id)) existing.push(id);
    }
    map.set(text, existing);
  }
  return map;
}

/** Line-wise {@link stripGroupTags}, for parsing another marker from raw markdown. */
export function stripGroupTagsFromMarkdown(md: string): string {
  return md.split("\n").map(stripGroupTags).join("\n");
}

/**
 * Deep copy of a tree with the `#group/` tags removed from every node label, so
 * the token never reaches the rendered map.
 */
export function stripGroupTagsDeep(node: MindNode): MindNode {
  return {
    ...node,
    content: stripGroupTags(node.content),
    children: node.children.map(stripGroupTagsDeep),
  };
}

/**
 * Collect groups from an already-transformed tree: every node whose plain text
 * carries a `#group/` membership is added, regardless of position, so members
 * may be non-consecutive. Groups are returned in first-seen order.
 */
export function annotateGroups(root: MindNode, parsed: Map<string, string[]>): Group[] {
  const order: string[] = [];
  const groups = new Map<string, Group>();
  const visit = (node: MindNode) => {
    for (const child of node.children) {
      const ids = parsed.get(stripGroupTags(boundaryText(nodeText(child))));
      if (ids) {
        for (const id of ids) {
          let group = groups.get(id);
          if (!group) {
            group = { id, nodes: [] };
            groups.set(id, group);
            order.push(id);
          }
          group.nodes.push(child);
        }
      }
      visit(child);
    }
  };
  visit(root);
  return order.map((id) => groups.get(id)!);
}
