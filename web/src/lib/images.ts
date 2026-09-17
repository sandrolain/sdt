import { normalizeCorpusPath } from "./exclusions";

/** URL-encode a corpus-relative path for the read-only /api/file endpoint. */
export function fileUrl(path: string): string {
  return `/api/file?path=${encodeURIComponent(path)}`;
}

/**
 * Resolve a frontmatter `image:` value to an /api/file URL. Absolute http(s)
 * URLs pass through; corpus-relative and document-relative paths resolve under
 * `context/`.
 */
export function imageUrl(value: string, basePath?: string): string {
  const raw = value.trim().replace(/^["']|["']$/g, "");
  if (!raw) return "";
  if (/^https?:\/\//i.test(raw)) return raw;
  let rel = raw.replace(/^\//, "");
  if (!rel.startsWith("context/")) {
    const dir =
      basePath && basePath.includes("/") ? basePath.slice(0, basePath.lastIndexOf("/")) : "";
    rel = dir ? `${dir}/${rel}` : `context/${rel}`;
  }
  return fileUrl(normalizeCorpusPath(rel));
}
