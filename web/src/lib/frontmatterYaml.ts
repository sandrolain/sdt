/** Real YAML parsing of a frontmatter block for the metadata panel. */

import { pathLabel, type FrontmatterField } from "./frontmatter";

/** Parsed fields, or a parse error to surface instead of dropping keys. */
export type FrontmatterParse = { fields: FrontmatterField[] } | { error: string };

// Session cache keyed by the raw frontmatter string, so re-renders and repeated
// documents neither re-import the lazy `yaml` chunk nor re-parse the block.
const cache = new Map<string, Promise<FrontmatterParse>>();

/** Test-only cache reset. */
export function clearFrontmatterCache(): void {
  cache.clear();
}

/** Parse + flatten a frontmatter block with the `yaml` library (lazy chunk). */
export function loadFrontmatter(frontmatter: string): Promise<FrontmatterParse> {
  const cached = cache.get(frontmatter);
  if (cached) return cached;
  const pending = parse(frontmatter);
  cache.set(frontmatter, pending);
  return pending;
}

async function parse(frontmatter: string): Promise<FrontmatterParse> {
  try {
    // dynamic import keeps the ~yaml chunk off first paint (the panel upgrades
    // from the tolerant parse once the chunk resolves)
    const { parseAllDocuments } = await import("yaml");
    const doc = parseAllDocuments(frontmatter)[0];
    if (!doc) return { fields: [] };
    if (doc.errors.length > 0) return { error: doc.errors[0].message };
    const root: unknown = doc.toJS();
    if (root === null || root === undefined) return { fields: [] };
    // the frontmatter must be a mapping; a scalar/list block is malformed
    if (typeof root !== "object" || Array.isArray(root)) {
      return { error: "frontmatter is not a mapping" };
    }
    return { fields: flatten(root as Record<string, unknown>) };
  } catch (err) {
    return { error: err instanceof Error ? err.message : String(err) };
  }
}

/** Depth-first flatten: leaves become rows, keyed by their full path. */
function flatten(root: Record<string, unknown>): FrontmatterField[] {
  const out: FrontmatterField[] = [];
  for (const [key, value] of Object.entries(root)) collect([key], value, out);
  return out;
}

function collect(path: string[], value: unknown, out: FrontmatterField[]): void {
  if (Array.isArray(value)) {
    const scalars: string[] = [];
    for (const item of value) {
      if (item !== null && typeof item === "object") out.push(makeField(path, [compact(item)]));
      else scalars.push(text(item));
    }
    if (scalars.length > 0) out.push(makeField(path, scalars));
    return;
  }
  if (value !== null && typeof value === "object") {
    for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
      collect([...path, key], child, out);
    }
    return;
  }
  out.push(makeField(path, [text(value)]));
}

function makeField(path: string[], values: string[]): FrontmatterField {
  return { path, key: path[path.length - 1], label: pathLabel(path), values };
}

function text(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "object") return compact(value);
  return String(value);
}

function compact(value: unknown): string {
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}
