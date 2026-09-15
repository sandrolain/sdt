/** Display-title helpers shared across components. */

/** Frontmatter `title` value, with surrounding quotes stripped, or "" when absent. */
export function frontmatterTitle(frontmatter?: string): string {
  const match = frontmatter?.match(/^title:\s*(.+)$/m)?.[1];
  return match?.trim().replace(/^["']|["']$/g, "") ?? "";
}

/** Strip a single wrapping pair of "double" or 'single' quotes. */
export function unwrapQuotes(value: string): string {
  return value.replace(/^["']|["']$/g, "").trim();
}

/** First `#` heading of a raw markdown document, quotes stripped, or "" when absent. */
export function firstH1(markdown?: string | null): string {
  const line = markdown?.match(/^#\s+(.+)$/m)?.[1];
  return line ? unwrapQuotes(line) : "";
}

/** Drop a leading `YYYYMMDD-HHMMSS-` filename prefix, only when it is a real date-lead. */
export function stripDatePrefix(name: string): string {
  return name.replace(/^\d{8}-\d{6}-/, "");
}

/** `YYYY-MM-DD` date embedded as a filename prefix, or "" when absent. */
export function filenameDate(path: string): string {
  const base = path.split("/").filter(Boolean).pop() ?? "";
  const match = /^(\d{4})(\d{2})(\d{2})-\d{6}-/.exec(base);
  return match ? `${match[1]}-${match[2]}-${match[3]}` : "";
}

/** Lightweight fallback title used when a document has no frontmatter title. */
export function fallbackTitle(path: string): string {
  const base = path.split("/").filter(Boolean).pop() ?? path;
  return base.endsWith(".md") ? base.slice(0, -3) : base;
}

/** Format a corpus path into a display title: basename, `.md` drop, date-prefix
 * trim, hyphens → spaces, quotes stripped, first letter uppercased. */
export function formatFilename(path: string): string {
  const base = path.split("/").filter(Boolean).pop() ?? path;
  const noExt = base.endsWith(".md") ? base.slice(0, -3) : base;
  const cleaned = unwrapQuotes(stripDatePrefix(noExt).replace(/-/g, " "));
  if (!cleaned) return base;
  return cleaned.charAt(0).toUpperCase() + cleaned.slice(1);
}

export interface TitleSource {
  /** explicit / frontmatter title */
  title?: string | null;
  /** raw markdown, used to extract a first H1 when no title is given */
  markdown?: string | null;
  /** corpus-relative path (or id) used as the final fallback */
  path?: string | null;
}

/** Shared display-title cascade: frontmatter title → first H1 → formatted filename. */
export function displayTitle({ title, markdown, path }: TitleSource): string {
  if (title && title.trim()) return unwrapQuotes(title);
  const h1 = firstH1(markdown);
  if (h1) return h1;
  if (path) return formatFilename(path);
  return "";
}