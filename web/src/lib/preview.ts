import { stripFrontmatter } from "./outline";
import { renderMarkdown } from "./markdown";

/** Session cache of rendered preview HTML, keyed by corpus path. */
const cache = new Map<string, string>();

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

/** Sanitized preview HTML: the leading body prose of a markdown document. */
export function previewHtml(markdown: string, limit = 1200): string {
  const body = stripFrontmatter(markdown).trim();
  const clipped = body.length > limit ? `${body.slice(0, limit)}\n\n…` : body;
  return renderMarkdown(clipped);
}

/** Fetch + render a document preview, cached by path. Aborts between hovers. */
export async function loadPreview(path: string, signal: AbortSignal): Promise<string> {
  const cached = cache.get(path);
  if (cached !== undefined) return cached;
  const res = await fetch(`/api/doc?path=${encodeURIComponent(path)}`, { signal });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const doc = (await res.json()) as { markdown?: string };
  if (signal.aborted) throw new DOMException("aborted", "AbortError");
  const html = previewHtml(doc.markdown ?? "");
  cache.set(path, html);
  return html;
}
