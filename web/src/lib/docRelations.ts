import type { CorpusIndex } from "./corpusIndex";
import { displayTitle } from "./titles";

/** The typed relation a document offers as an ancestor link. */
export interface Relation {
  label: string;
  path: string;
  title: string;
}

/**
 * The ancestor this document derives from: a task file names its plan, a plan
 * its analysis. Resolved from the tree payload's typed edge, so a surface never
 * invents a hierarchy the corpus does not declare.
 */
export function relationFor(path: string, index: CorpusIndex | null): Relation | null {
  const entry = index?.get(path)?.entry;
  if (!entry) return null;
  const parent = entry.plan ?? entry.analysis;
  if (!parent) return null;
  const label = entry.plan ? "Plan" : "Analysis";
  const parentEntry = index?.get(parent)?.entry;
  return {
    label,
    path: parent,
    title: displayTitle({ title: parentEntry?.title, path: parent }),
  };
}

/** The other documents in the same folder, title-sorted. */
export function siblingsOf(
  path: string,
  index: CorpusIndex | null,
): { path: string; title: string }[] {
  if (!index) return [];
  const folder = path.slice(0, path.lastIndexOf("/") + 1);
  const out: { path: string; title: string }[] = [];
  for (const [entryPath, info] of index) {
    if (entryPath === path || !entryPath.startsWith(folder)) continue;
    out.push({
      path: entryPath,
      title: displayTitle({ title: info.entry.title, path: entryPath }),
    });
  }
  return out.sort((a, b) => a.title.localeCompare(b.title));
}
