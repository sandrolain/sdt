import { stripLeadingH1 } from "./headings";

/** One node in the Map-mode outline built from headings and list items. */
export interface OutlineItem {
  /** abstract depth: 1-6 for headings, 7+ for list items by indentation */
  depth: number;
  kind: "heading" | "list";
  text: string;
  children: OutlineItem[];
}

const HEADING_RE = /^(#{1,6})\s+(.*?)\s*$/;
const LIST_RE = /^(\s*)(?:[-*+]|\d+\.)\s+(.*?)\s*$/;

/** Drop a leading YAML frontmatter block. */
export function stripFrontmatter(md: string): string {
  if (!md.startsWith("---\n")) return md;
  const end = md.indexOf("\n---", 4);
  if (end < 0) return md;
  const after = md.indexOf("\n", end + 1);
  return after < 0 ? "" : md.slice(after + 1);
}

/** Drop fenced code blocks so their content is not mistaken for structure. */
function stripFences(md: string): string {
  const lines = md.split("\n");
  const out: string[] = [];
  let fence: string | null = null;
  for (const line of lines) {
    const marker = /^\s*(```|~~~)/.exec(line);
    if (marker) {
      fence = fence ? null : marker[1];
      continue;
    }
    if (fence) continue;
    out.push(line);
  }
  return out.join("\n");
}

/**
 * Build an outline tree from headings (depth = level) and list items
 * (depth = 7 + indentation level) for ordinary markdown Map mode.
 */
export function parseOutline(md: string): OutlineItem[] {
  const lines = stripFences(stripLeadingH1(stripFrontmatter(md))).split("\n");
  const root: OutlineItem = { depth: 0, kind: "heading", text: "", children: [] };
  const stack: OutlineItem[] = [root];
  for (const line of lines) {
    const heading = HEADING_RE.exec(line);
    const list = heading ? null : LIST_RE.exec(line);
    if (!heading && !list) continue;
    const item: OutlineItem = heading
      ? { depth: heading[1].length, kind: "heading", text: heading[2], children: [] }
      : { depth: 7 + Math.floor(list![1].length / 2), kind: "list", text: list![2], children: [] };
    while (stack.length > 1 && stack[stack.length - 1].depth >= item.depth) stack.pop();
    stack[stack.length - 1].children.push(item);
    stack.push(item);
  }
  return root.children;
}
