import { docHref, resolveDocPath, wikiHref, type WikiIndex } from "./wikiLinks";
import { isExcludedPath, normalizeCorpusPath } from "./exclusions";
import { displayTitle } from "./titles";

export interface DocLink {
  /** visible label */
  label: string;
  /** navigation target; absent for name-only (excluded/unknown) links */
  href?: string;
  /** true when the target exists but is excluded from the corpus */
  excluded?: boolean;
  /** true for http(s) targets (rendered as plain links) */
  external?: boolean;
}

/** Resolve a corpus path target (`foo.md`, `context/foo.md`) to a Docs route. */
export function resolveCorpusDocHref(path: string, basePath: string): string {
  let rel = path.trim().replace(/^\//, "");
  if (!rel.startsWith("context/")) {
    rel = rel.includes("/") ? `context/${rel}` : resolveDocPath(rel, basePath);
  }
  return docHref(normalizeCorpusPath(rel));
}

/** True for the string form of an already-linked target (`[[id|label]]` / `[label](href)`). */
function wikilinkId(value: string): string | null {
  const m = /^\[\[([^\]]+)\]\]$/.exec(value.trim());
  if (!m) return null;
  const inner = m[1];
  const head = inner.includes("|") ? inner.slice(0, inner.indexOf("|")).trim() : inner.trim();
  const sep = head.indexOf("::");
  return (sep >= 0 ? head.slice(sep + 2) : head).trim() || null;
}

/** Resolve an id or title against the wiki index to `{ id, label }`. */
function resolveWikiTarget(value: string, index?: WikiIndex): { id: string; label: string } | null {
  if (!index) return null;
  if (index.byId.has(value)) return { id: value, label: index.byId.get(value) ?? value };
  const id = index.byTitle.get(value);
  if (id) return { id, label: index.byId.get(id) ?? value };
  return null;
}

/** Resolve one frontmatter/body target into a navigable or name-only link. */
export function resolveDocLink(target: string, basePath: string, index?: WikiIndex): DocLink {
  const raw = target.trim().replace(/^["']|["']$/g, "");
  if (raw === "") return { label: target, excluded: true };
  if (/^https?:\/\//i.test(raw)) return { label: raw, href: raw, external: true };

  const wikiId = wikilinkId(raw);
  if (wikiId !== null) {
    const resolved = resolveWikiTarget(wikiId, index);
    return resolved ? { label: resolved.label, href: wikiHref(resolved.id) } : { label: wikiId };
  }

  if (raw.endsWith(".md")) {
    const rel = raw.startsWith("context/") ? raw : raw.includes("/") ? `context/${raw}` : raw;
    const normalized = normalizeCorpusPath(rel);
    const label = displayTitle({ path: normalized });
    if (isExcludedPath(normalized)) return { label, excluded: true };
    return { label, href: resolveCorpusDocHref(raw, basePath) };
  }

  const resolved = resolveWikiTarget(raw, index);
  if (resolved) return { label: resolved.label, href: wikiHref(resolved.id) };
  return { label: raw };
}

const BODY_WIKILINK = /\[\[([^\]]+)\]\]/g;
const BODY_MD_LINK = /\[([^\]]*)\]\(([^)\s]+\.md)(?:#[^)]*)?\)/g;

/** Link targets collected from the document body (`.md` links and `[[…]]`). */
export function collectBodyLinks(markdown: string, basePath: string, index?: WikiIndex): DocLink[] {
  const links: DocLink[] = [];
  for (const m of markdown.matchAll(BODY_WIKILINK)) {
    links.push(resolveDocLink(m[0], basePath, index));
  }
  for (const m of markdown.matchAll(BODY_MD_LINK)) {
    links.push(resolveDocLink(m[2], basePath, index));
  }
  return links;
}

/** Link targets collected from frontmatter `links`, `sources` and `relations` fields. */
export function collectMetaLinks(
  fields: { key: string; values: string[] }[],
  basePath: string,
  index?: WikiIndex,
): DocLink[] {
  const keys = new Set(["links", "sources", "relations"]);
  const links: DocLink[] = [];
  for (const field of fields) {
    if (!keys.has(field.key)) continue;
    for (const value of field.values) links.push(resolveDocLink(value, basePath, index));
  }
  return links;
}
