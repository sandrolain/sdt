import { Marked, type Token, type Tokens } from "marked";
import {
  extractMarkerTitles,
  parseNodeMarkers,
  type MarkerTitles,
  type NodeMarkers,
} from "./mapMarkers";
import { stripFrontmatter } from "./outline";
import { rewriteDocLinks, rewriteWikiLinks, type WikiIndex } from "./wikiLinks";

/** Structural node shape shared by the map tree, the fuse pass and the overlays. */
export interface MindNode {
  content: string;
  children: MindNode[];
  payload?: MindNodePayload;
}

/** The block a node came from; drives how the label is styled and sized. */
export type MapNodeKind = "topic" | "quote" | "code" | "table" | "text";

export interface MindNodePayload {
  kind: MapNodeKind;
  /** XMindMark markers read from this node's own text (plan D4). */
  markers?: NodeMarkers;
  /** Boundary and summary titles; set on the root node only. */
  titles?: MarkerTitles;
}

export interface MindmapOptions {
  basePath?: string;
  wikiIndex?: WikiIndex;
}

const HTML_ESCAPES: Record<string, string> = {
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  '"': "&quot;",
  "'": "&#39;",
};

function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, (c) => HTML_ESCAPES[c]);
}

/**
 * The map's own `marked` instance: inline markdown in a label becomes HTML
 * (a node label is HTML by contract and is sanitized when it renders), and raw
 * HTML is escaped rather than passed through, so a map document can never
 * inject markup into the viewer.
 */
const marked = new Marked({
  renderer: { html: (token: { text: string }) => escapeHtml(token.text) },
});

/** Inline markdown (after the link rewrite) to the HTML stored in `content`. */
function inline(text: string): string {
  return marked.parseInline(text) as string;
}

const FRONTMATTER_TITLE_RE = /^title:[ \t]*(.*?)[ \t]*$/m;

/** The frontmatter `title`, the root fallback when a map has no `#` heading. */
function frontmatterTitle(md: string): string {
  if (!md.startsWith("---\n")) return "";
  const end = md.indexOf("\n---", 4);
  if (end < 0) return "";
  const match = FRONTMATTER_TITLE_RE.exec(md.slice(4, end));
  if (!match) return "";
  return match[1].replace(/^["']|["']$/g, "");
}

function node(content: string, kind: MapNodeKind): MindNode {
  return { content, children: [], payload: { kind } };
}

/** Build a node from raw markdown: markers are read and stripped from the label. */
function fromMarkdown(text: string, kind: MapNodeKind): MindNode {
  const { label, markers } = parseNodeMarkers(text);
  return { ...node(inline(label), kind), payload: { kind, markers } };
}

/** Open heading chain; the last entry is the block parent, `fallback` the root. */
interface StackEntry {
  level: number;
  node: MindNode;
}

/**
 * Table cells and quoted lines are **content leaves**, not topics: a marker
 * written inside them stays literal, so the two implementations (the SPA
 * parser and the Go lint) agree on which node carries which marker.
 */
function tableNode(table: Tokens.Table): MindNode {
  const raw = [table.header, ...table.rows]
    .map((cells) => cells.map((c) => c.text).join(" · "))
    .join("\n");
  return node(
    raw
      .split("\n")
      .map((line) => escapeHtml(line))
      .join("<br>"),
    "table",
  );
}

/**
 * A blockquote becomes one `quote` node: its paragraphs form the label (one
 * line each) and its nested lists stay its children.
 */
function blockquoteNode(quote: Tokens.Blockquote): MindNode {
  const raw = (quote.tokens ?? [])
    .filter((t) => t.type === "paragraph")
    .map((t) => (t as Tokens.Paragraph).text)
    .join("\n");
  return node(
    raw
      .split("\n")
      .map((line) => inline(line))
      .join("<br>"),
    "quote",
  );
}

/** List items become nodes; a nested list or a trailing paragraph is a child. */
function attachList(list: Tokens.List, parent: MindNode): void {
  for (const item of list.items) {
    const label: string[] = [];
    const rest: Token[] = [];
    let labelled = false;
    for (const token of item.tokens ?? []) {
      // The GFM checkbox is a token of its own; its state travels in the label.
      if (token.type === "checkbox") continue;
      if (!labelled && (token.type === "text" || token.type === "paragraph")) {
        label.push(token.text);
        labelled = true;
        continue;
      }
      rest.push(token);
    }
    const prefix = item.task ? (item.checked ? "[x] " : "[ ] ") : "";
    const text = `${prefix}${label.join(" ").trim()}`.trim();
    const itemNode = text ? fromMarkdown(text, "topic") : null;
    if (itemNode) parent.children.push(itemNode);
    attachBlocks(rest, itemNode ?? parent, []);
  }
}

/**
 * Walk block tokens into `parent`, nesting headings by level. `stack` carries
 * the open headings across the sibling blocks of one scope; a block that is not
 * a heading attaches to the innermost open heading (or the scope's parent).
 */
function attachBlocks(tokens: Token[], parent: MindNode, stack: StackEntry[]): void {
  const current = () => (stack.length > 0 ? stack[stack.length - 1].node : parent);
  for (const token of tokens) {
    switch (token.type) {
      case "heading": {
        const { depth } = token as Tokens.Heading;
        while (stack.length > 0 && stack[stack.length - 1].level >= depth) stack.pop();
        const child = fromMarkdown(token.text, "topic");
        current().children.push(child);
        stack.push({ level: depth, node: child });
        break;
      }
      case "list":
        attachList(token as Tokens.List, current());
        break;
      case "paragraph": {
        const text = (token as Tokens.Paragraph).text;
        if (text.trim()) current().children.push(fromMarkdown(text, "topic"));
        break;
      }
      case "blockquote": {
        const quote = token as Tokens.Blockquote;
        const child = blockquoteNode(quote);
        current().children.push(child);
        attachBlocks(
          (quote.tokens ?? []).filter((t) => t.type !== "paragraph"),
          child,
          [],
        );
        break;
      }
      case "code": {
        const { text } = token as Tokens.Code;
        if (text.trim()) current().children.push(node(escapeHtml(text), "code"));
        break;
      }
      case "table":
        current().children.push(tableNode(token as Tokens.Table));
        break;
      case "space":
        break;
      default: {
        // An unrecognised block keeps its raw source as a text leaf: the parser
        // never drops content silently.
        const raw = (token as { raw?: string }).raw ?? "";
        if (raw.trim()) current().children.push(node(escapeHtml(raw.trim()), "text"));
      }
    }
  }
}

/**
 * Parse a `.map.md` body into the map tree: the `marked` lexer supplies the
 * block structure (headings, lists, paragraphs, blockquotes, code, tables) and
 * every block becomes a node instead of being discarded. Boundary and summary
 * title lines are lifted out first, so a title is never a topic; frontmatter is
 * stripped and both link syntaxes are rewritten to app routes, so `content`
 * holds ready-to-render HTML and every node carries its own markers.
 */
export function parseMapDocument(md: string, opts: MindmapOptions = {}): MindNode {
  const { titles, body: withoutTitles } = extractMarkerTitles(stripFrontmatter(md));
  const prepared = rewriteDocLinks(rewriteWikiLinks(withoutTitles, opts.wikiIndex), opts.basePath);
  const tokens = marked.lexer(prepared);
  const rootIndex = tokens.findIndex(
    (t) => t.type === "heading" && (t as Tokens.Heading).depth === 1,
  );
  const heading = rootIndex >= 0 ? (tokens[rootIndex] as Tokens.Heading).text : undefined;
  const title = frontmatterTitle(md);
  const root =
    heading !== undefined
      ? fromMarkdown(heading, "topic")
      : node(title ? fromMarkdown(title, "topic").content : "", "topic");
  const stack: StackEntry[] = heading !== undefined ? [{ level: 1, node: root }] : [];
  attachBlocks(tokens.slice(rootIndex + 1), root, stack);
  return {
    ...root,
    payload: { kind: root.payload?.kind ?? "topic", markers: root.payload?.markers, titles },
  };
}
