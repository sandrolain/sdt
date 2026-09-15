import type { MindNode } from "./mindmap";

export interface BoundaryMark {
  /** plain topic text → boundary id */
  mark: Map<string, string>;
  /** boundary id → optional title */
  titles: Map<string, string>;
}

export interface BoundaryGroup {
  id: string;
  title?: string;
  nodes: MindNode[];
}

export interface BoundaryRect {
  id: string;
  title?: string;
  x: number;
  y: number;
  width: number;
  height: number;
}

const TITLE_LINE_RE = /^\s*\[B(\d*)\]:\s*(.*?)\s*$/;
const MARK_RE = /\[B(\d*)\]/g;

/** Plain topic text: strip list/heading prefixes and XMindMark markers. */
export function boundaryText(line: string): string {
  return line
    .replace(/^\s*#{1,6}\s+/, "")
    .replace(/^\s*(?:[-*+]|\d+\.)\s+/, "")
    .replace(MARK_RE, "")
    .replace(/^\*\*(.*)\*\*$/, "$1")
    .trim();
}

/**
 * Parse XMindMark boundary syntax from page markdown:
 * `[B]`/`[B<n>]` appended to a topic marks membership; `[B<n>]: title` lines
 * name the boundary. Boundary titles are not topics themselves.
 */
export function parseBoundaries(md: string): BoundaryMark {
  const mark = new Map<string, string>();
  const titles = new Map<string, string>();
  for (const raw of md.split("\n")) {
    const title = TITLE_LINE_RE.exec(raw);
    if (title) {
      titles.set(title[1] || "0", title[2]);
      continue;
    }
    const idMatch = /\[B(\d*)\]/.exec(raw);
    if (!idMatch) continue;
    const text = boundaryText(raw);
    if (text) mark.set(text, idMatch[1] || "0");
  }
  return { mark, titles };
}

/** Text content of a node, with HTML tags stripped. */
export function nodeText(node: MindNode): string {
  return node.content
    .replace(/<[^>]*>/g, "")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .trim();
}

/**
 * Collect boundary groups: consecutive sibling nodes sharing the same
 * boundary id (one group level, per XMind semantics).
 */
export function annotateBoundaries(root: MindNode, parsed: BoundaryMark): BoundaryGroup[] {
  const groups: BoundaryGroup[] = [];
  const visit = (node: MindNode) => {
    let current: BoundaryGroup | null = null;
    let currentId: string | null = null;
    for (const child of node.children) {
      const id = parsed.mark.get(nodeText(child));
      if (id !== undefined && id === currentId) {
        current?.nodes.push(child);
      } else if (id !== undefined) {
        current = { id, title: parsed.titles.get(id), nodes: [child] };
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

/** Union rects (layout coords) for each group, expanded by padding. */
export function boundaryRects(groups: BoundaryGroup[], padding = 8): BoundaryRect[] {
  const out: BoundaryRect[] = [];
  for (const group of groups) {
    const rects = group.nodes
      .map((n) => n.state?.rect)
      .filter((r): r is NonNullable<typeof r> => Boolean(r));
    if (rects.length === 0) continue;
    const x = Math.min(...rects.map((r) => r.x)) - padding;
    const y = Math.min(...rects.map((r) => r.y)) - padding;
    const right = Math.max(...rects.map((r) => r.x + r.width)) + padding;
    const bottom = Math.max(...rects.map((r) => r.y + r.height)) + padding;
    out.push({ id: group.id, title: group.title, x, y, width: right - x, height: bottom - y });
  }
  return out;
}
