import type { NodeMarkers } from "./mapMarkers";
import type { MapNodeKind, MindNode } from "./mindmap";

/**
 * Map geometry is **computed, never measured** (plan D7): a node's box comes
 * from a character-metric estimate over its label, and the renderer draws the
 * node at exactly that box. Layout, overlays and the exporter therefore share
 * one geometry model, and every claim about them is deterministic under jsdom —
 * only the browser pass can claim pixels.
 */

/** Fixed metrics, in CSS pixels, shared by the layouts and the exporter. */
export const METRICS = {
  /** average glyph advance of the reading font at NODE_FONT_SIZE */
  charWidth: 7.1,
  nodeFontSize: 13,
  lineHeight: 20,
  paddingX: 10,
  paddingY: 8,
  /** node width per kind; a fixed width keeps the box predictable */
  widthTopic: 200,
  widthRoot: 240,
  widthQuote: 240,
  widthCode: 260,
  widthTable: 240,
  minHeight: 30,
  /** gap between sibling boxes */
  siblingGap: 14,
  /** gap between depth columns (balanced) and rings (radial) */
  depthGap: 56,
  /** padding around a boundary/summary/hull shape */
  shapePadding: 10,
} as const;

export interface MapBox {
  width: number;
  height: number;
}

/** One node with its stable id and its computed box: the layout's input. */
export interface MeasuredNode {
  /** DFS path: the root is `n0`, its first child `n0.1` */
  id: string;
  content: string;
  kind: MapNodeKind;
  depth: number;
  markers: NodeMarkers;
  box: MapBox;
  /** plain text of the label, one entry per rendered line */
  lines: string[];
  /** hrefs the label links to, for click-to-open */
  links: string[];
  parent: string | null;
  children: MeasuredNode[];
}

const EMPTY_MARKERS: NodeMarkers = { notes: [], folded: false, stickers: [], groups: [] };

/** Node width by kind; the root is a topic and gets the wider box. */
function widthFor(kind: MapNodeKind, depth: number): number {
  if (kind === "code") return METRICS.widthCode;
  if (kind === "quote" || kind === "table") return METRICS.widthQuote;
  return depth === 0 ? METRICS.widthRoot : METRICS.widthTopic;
}

/**
 * The rendered lines of a label: tags stripped, `<br>` kept as a line break.
 * A label with no text (a code block, a title-only node) still yields one line
 * so the box has a height.
 */
export function labelLines(html: string): string[] {
  const lines = html
    .split(/<br\s*\/?>/i)
    .map((part) =>
      part
        .replace(/<[^>]*>/g, " ")
        .replace(/&amp;/g, "&")
        .replace(/&lt;/g, "<")
        .replace(/&gt;/g, ">")
        .replace(/&quot;/g, '"'),
    )
    .map((part) => part.replace(/\s+/g, " ").trim());
  return lines.length > 0 ? lines : [""];
}

/** Wrap one line to the node's character budget, never mid-word when avoidable. */
export function wrapLabel(line: string, maxChars: number): string[] {
  if (line.length <= maxChars) return [line];
  const words = line.split(/\s+/).filter(Boolean);
  if (words.length === 0) return [line];
  const out: string[] = [];
  let current = "";
  for (const word of words) {
    if (!current) {
      current = word;
    } else if (`${current} ${word}`.length <= maxChars) {
      current = `${current} ${word}`;
    } else {
      out.push(current);
      current = word;
    }
  }
  if (current) out.push(current);
  return out;
}

/** The box of a label at a given kind and depth. */
export function measureNode(
  html: string,
  kind: MapNodeKind,
  depth: number,
): MapBox & { lines: string[] } {
  const width = widthFor(kind, depth);
  const maxChars = Math.max(8, Math.floor((width - 2 * METRICS.paddingX) / METRICS.charWidth));
  const lines = labelLines(html).flatMap((line) => wrapLabel(line, maxChars));
  const height = Math.max(
    METRICS.minHeight,
    lines.length * METRICS.lineHeight + 2 * METRICS.paddingY,
  );
  return { width, height, lines };
}

const HREF_RE = /href="([^"]+)"/g;

/** Build the measured tree: stable ids, computed boxes, label lines and links. */
export function measureTree(root: MindNode): MeasuredNode {
  const visit = (
    node: MindNode,
    id: string,
    depth: number,
    parent: string | null,
  ): MeasuredNode => {
    const kind = node.payload?.kind ?? "topic";
    const box = measureNode(node.content, kind, depth);
    const links: string[] = [];
    HREF_RE.lastIndex = 0;
    for (let m = HREF_RE.exec(node.content); m; m = HREF_RE.exec(node.content)) {
      links.push(m[1]);
    }
    return {
      id,
      content: node.content,
      kind,
      depth,
      markers: node.payload?.markers ?? EMPTY_MARKERS,
      box: { width: box.width, height: box.height },
      lines: box.lines,
      links,
      parent,
      children: node.children.map((child, i) => visit(child, `${id}.${i + 1}`, depth + 1, id)),
    };
  };
  return visit(root, "n0", 0, null);
}

/** Depth-first flatten of a measured tree, parents before children. */
export function flattenMeasured(node: MeasuredNode): MeasuredNode[] {
  return [node, ...node.children.flatMap(flattenMeasured)];
}

/** Text of a measured label, for a shape title or an export. */
export function measuredText(node: MeasuredNode): string {
  return node.lines.join(" ");
}
