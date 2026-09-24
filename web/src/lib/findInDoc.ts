/**
 * Client-only find-in-document helpers: collect case-insensitive match ranges
 * over the text nodes of a rendered document and wrap them in highlight marks.
 * Pure DOM utilities, unit-tested with jsdom.
 *
 * Skipped subtrees: `svg` (rendered diagrams) and the math/mermaid placeholders
 * (`.md-math`, `.md-mermaid`) whose content is plain text, not the doc prose.
 */

export interface FindRange {
  node: Text;
  start: number;
  end: number;
  index: number;
}

const SVG_NS = "http://www.w3.org/2000/svg";
const SKIP_CLASSES = ["md-math", "md-mermaid"];

function blockReject(node: Node): boolean {
  let el = node.parentElement;
  while (el) {
    if (el.namespaceURI === SVG_NS || hasSkipClass(el)) {
      return true;
    }
    el = el.parentElement;
  }
  return false;
}

function hasSkipClass(el: Element): boolean {
  return SKIP_CLASSES.some((c) => el.classList.contains(c));
}

/** Lists every case-insensitive substring match of the query in root. */
export function collectFindRanges(root: Node, query: string): FindRange[] {
  const q = query.trim().toLowerCase();
  if (!q || !root) return [];
  const ranges: FindRange[] = [];
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      return blockReject(node) ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_ACCEPT;
    },
  });
  let node: Node | null;
  let index = 0;
  while ((node = walker.nextNode()) !== null) {
    const text = (node.nodeValue ?? "").toLowerCase();
    let cursor = 0;
    let at = text.indexOf(q, cursor);
    while (at >= 0) {
      ranges.push({ node: node as Text, start: at, end: at + q.length, index: index++ });
      cursor = at + q.length;
      at = text.indexOf(q, cursor);
    }
  }
  return ranges;
}

/** Wraps each range in a `<mark class="find-hit">`; the active one gets `is-current`. */
export function applyFindHighlights(root: HTMLElement, ranges: FindRange[], current: number): void {
  clearFindHighlights(root);
  if (ranges.length === 0) return;
  const byNode = new Map<Text, FindRange[]>();
  for (const range of ranges) {
    const list = byNode.get(range.node);
    if (list) list.push(range);
    else byNode.set(range.node, [range]);
  }
  // Process later offsets first so earlier splits keep their node offsets valid.
  for (const group of byNode.values()) {
    group.sort((a, b) => b.start - a.start);
    for (const range of group) {
      replaceWithMark(range, range.index === current);
    }
  }
}

function replaceWithMark(range: FindRange, isCurrent: boolean): void {
  const { node, start, end } = range;
  const parent = node.parentNode;
  if (!parent) return;
  node.splitText(end);
  const mid = node.splitText(start);
  const mark = document.createElement("mark");
  mark.className = `find-hit${isCurrent ? " is-current" : ""}`;
  parent.replaceChild(mark, mid);
  mark.appendChild(mid);
}

/** Removes every find highlight, merging the surrounding text nodes back. */
export function clearFindHighlights(root: ParentNode): void {
  for (const mark of root.querySelectorAll("mark.find-hit")) {
    const parent = mark.parentNode;
    if (!parent) continue;
    while (mark.firstChild) parent.insertBefore(mark.firstChild, mark);
    parent.removeChild(mark);
    parent.normalize();
  }
}

/** Scrolls the current match into the viewport center (no-op when absent). */
export function scrollFindCurrent(root: ParentNode): void {
  root.querySelector("mark.find-hit.is-current")?.scrollIntoView({ block: "center" });
}
