import { parseFrontmatter } from "./frontmatter";
import { displayTitle } from "./titles";

/** Metadata card shown on link hover. */
export interface PreviewMeta {
  title: string;
  summary: string;
  created: string;
  modified: string;
  /** frontmatter `image` value, unresolved (see `imageUrl`) */
  image: string;
  path: string;
}

/** Session cache of preview metadata, keyed by corpus path. */
const cache = new Map<string, PreviewMeta>();

/** Test-only cache reset. */
export function clearPreviewCache(): void {
  cache.clear();
}

/** Corpus path targeted by a previewable in-document link, or null. */
export function previewPathFromHref(href: string): string | null {
  if (href.startsWith("#/docs/")) {
    const path = href.slice("#/docs/".length);
    return path ? decodeURIComponent(path) : null;
  }
  if (href.startsWith("#/wiki/")) {
    const id = href.slice("#/wiki/".length);
    if (!id || id === "graph" || id === "board") return null;
    return `context/wiki/${decodeURIComponent(id)}.md`;
  }
  return null;
}

/** Metadata parsed from a document's frontmatter and path. */
export function previewMeta(path: string, frontmatter?: string): PreviewMeta {
  const fields = parseFrontmatter(frontmatter);
  const value = (key: string) => fields.find((f) => f.key === key)?.values[0] ?? "";
  return {
    title: value("title") || displayTitle({ path }),
    summary: value("summary"),
    created: value("created"),
    modified: value("updated"),
    image: value("image"),
    path,
  };
}

/** Fetch + parse a document's preview metadata, cached by path. */
export async function loadPreview(path: string, signal: AbortSignal): Promise<PreviewMeta> {
  const cached = cache.get(path);
  if (cached !== undefined) return cached;
  const res = await fetch(`/api/doc?path=${encodeURIComponent(path)}`, { signal });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const doc = (await res.json()) as { frontmatter?: string; markdown?: string };
  if (signal.aborted) throw new DOMException("aborted", "AbortError");
  const meta = previewMeta(path, doc.frontmatter);
  cache.set(path, meta);
  return meta;
}
