/** Corpus paths excluded from tree/viewer/search (mirrors internal/corpus). */
const EXCLUDED_DIRS = new Set(["tmp", "scripts", "refs", "commands", "instructions", "sdtdocs"]);

/** Collapse `.`/`..` and normalize separators to a slash-separated relative path. */
export function normalizeCorpusPath(rel: string): string {
  const parts: string[] = [];
  for (const part of rel.replace(/\\/g, "/").split("/")) {
    if (part === "" || part === ".") continue;
    if (part === "..") parts.pop();
    else parts.push(part);
  }
  return parts.join("/");
}

/** True when a corpus-relative path is excluded (excluded dir segment or corpus README). */
export function isExcludedPath(rel: string): boolean {
  const clean = normalizeCorpusPath(rel);
  if (clean === "context/README.md") return true;
  return clean.split("/").some((seg) => EXCLUDED_DIRS.has(seg));
}
