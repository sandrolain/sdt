import type { TreeEntry } from "./api";

/** id → title and title → id lookups for wiki pages, derived from the tree. */
export interface WikiIndex {
  byId: Map<string, string>;
  byTitle: Map<string, string>;
}

/** Canonical wiki id for a corpus path: wiki-relative minus ".md". */
export function wikiIdFromPath(path: string): string {
  const prefix = "context/wiki/";
  const p = path.endsWith(".md") ? path.slice(0, -3) : path;
  return p.startsWith(prefix) ? p.slice(prefix.length) : p;
}

/** Build the id/title index from /api/tree entries (wiki dir only). */
export function buildWikiIndex(entries: TreeEntry[]): WikiIndex {
  const byId = new Map<string, string>();
  const byTitle = new Map<string, string>();
  for (const e of entries) {
    if (e.canvas || !e.path.startsWith("context/wiki/")) continue;
    const id = wikiIdFromPath(e.path);
    if (!byId.has(id)) byId.set(id, e.title ?? id);
    const title = e.title?.trim();
    if (title && !byTitle.has(title)) byTitle.set(title, id);
  }
  return { byId, byTitle };
}

export interface ParsedWikiLink {
  /** raw inner content, e.g. "depends_on::Auth service" */
  raw: string;
  /** closed-set relation verb when the link used the `verb::target` form */
  verb?: string;
  /** id or title the link points to */
  target: string;
  /** visible label */
  label: string;
}

/** Parse the inner text of a `[[…]]` wikilink. Returns null when malformed. */
export function parseWikiLink(raw: string): ParsedWikiLink | null {
  const inner = raw.trim();
  if (!inner) return null;
  const pipe = inner.indexOf("|");
  const head = pipe >= 0 ? inner.slice(0, pipe).trim() : inner;
  const explicitLabel = pipe >= 0 ? inner.slice(pipe + 1).trim() : "";
  if (!head) return null;
  const sep = head.indexOf("::");
  const verb = sep >= 0 ? head.slice(0, sep).trim() : undefined;
  const target = (sep >= 0 ? head.slice(sep + 2) : head).trim();
  if (!target) return null;
  return { raw: inner, verb: verb || undefined, target, label: explicitLabel || target };
}

/** Resolve a parsed wikilink to a wiki id, or null when it cannot be resolved. */
export function resolveWikiLink(link: ParsedWikiLink, index?: WikiIndex): string | null {
  if (!index) return link.verb ? null : link.target;
  return index.byId.get(link.target) !== undefined
    ? link.target
    : (index.byTitle.get(link.target) ?? (link.verb ? null : link.target));
}

/** Hash route for a wiki page id. */
export function wikiHref(id: string): string {
  return `#/wiki/${id}`;
}

/** Hash route for a corpus document path. */
export function docHref(path: string): string {
  return `#/docs/${path}`;
}

/**
 * Resolve a relative `.md` href against the containing document path
 * (`basePath`), collapsing `.`/`..`; absolute hrefs are corpus-relative.
 */
export function resolveDocPath(href: string, basePath?: string): string {
  const clean = href.replace(/^\.\//, "");
  if (clean.startsWith("/")) return clean.slice(1);
  const baseDir =
    basePath && basePath.includes("/") ? basePath.slice(0, basePath.lastIndexOf("/")) : "";
  const parts = `${baseDir}/${clean}`.split("/");
  const out: string[] = [];
  for (const part of parts) {
    if (part === "" || part === ".") continue;
    if (part === "..") out.pop();
    else out.push(part);
  }
  return out.join("/");
}

const WIKILINK_RE = /\[\[([^\]|]+)(?:\|([^\]]+))?\]\]/g;

/**
 * Rewrite `[[id]]`, `[[id|label]]` and `[[verb::title]]` occurrences into
 * markdown links to the wiki route. Unresolved links become inert
 * `span.wikilink-broken` elements (graceful error state, no navigation).
 */
export function rewriteWikiLinks(md: string, index?: WikiIndex): string {
  return md.replace(WIKILINK_RE, (match, head: string, pipeLabel?: string) => {
    const parsed = parseWikiLink(pipeLabel ? `${head}|${pipeLabel}` : head);
    if (!parsed) return escapeHtml(match);
    const id = resolveWikiLink(parsed, index);
    if (!id) {
      return `<span class="wikilink-broken" title="Unresolved link: ${escapeHtml(parsed.target)}">${escapeHtml(parsed.label)}</span>`;
    }
    return `[${parsed.label}](${wikiHref(id)})`;
  });
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}
