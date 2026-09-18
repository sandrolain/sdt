import type { TreeEntry } from "./api";
import { KIND_ORDER, entryKind, type EntryFilterKind } from "./kinds";
import { displayTitle } from "./titles";

export type TreeSortKey = "name" | "title" | "created" | "modified";
export type TreeDir = "asc" | "desc";

export const TREE_SORTS: { id: TreeSortKey; label: string }[] = [
  { id: "name", label: "Name" },
  { id: "title", label: "Title" },
  { id: "created", label: "Created" },
  { id: "modified", label: "Modified" },
];

export interface TreeGroup {
  kind: EntryFilterKind;
  entries: TreeEntry[];
}

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

function sortValue(entry: TreeEntry, key: TreeSortKey): string {
  switch (key) {
    case "name":
      return entry.path.split("/").filter(Boolean).pop() ?? entry.path;
    case "title":
      return displayTitle({ title: entry.title, path: entry.path });
    case "created":
      return entry.created ?? "";
    case "modified":
      return entry.modified ?? "";
  }
}

/** Sort entries with locale-aware collation; dates compare ISO-lexicographically.
 *  Entries missing the sort field stay last in both directions. */
export function sortEntries(entries: TreeEntry[], key: TreeSortKey, dir: TreeDir): TreeEntry[] {
  const sorted = [...entries].sort((a, b) => {
    const av = sortValue(a, key);
    const bv = sortValue(b, key);
    if (av === "" || bv === "") {
      if (av === bv) return collator.compare(a.path, b.path);
      return av === "" ? 1 : -1;
    }
    return collator.compare(av, bv);
  });
  if (dir === "asc") return sorted;
  const empty = sorted.filter((e) => sortValue(e, key) === "");
  return [...sorted.filter((e) => sortValue(e, key) !== "").reverse(), ...empty];
}

/** Group entries into kind folders, ordered by KIND_ORDER with canvas last.
 *  Every known kind is returned (empty groups allowed, count 0); the canvas
 *  group appears only when the corpus actually contains .canvas files. */
export function groupByKind(entries: TreeEntry[]): TreeGroup[] {
  const map = new Map<EntryFilterKind, TreeEntry[]>();
  for (const entry of entries) {
    const kind = entryKind(entry);
    const bucket = map.get(kind);
    if (bucket) bucket.push(entry);
    else map.set(kind, [entry]);
  }
  return [
    ...KIND_ORDER.map((kind) => ({ kind, entries: map.get(kind) ?? [] })),
    ...(map.has("canvas") ? [{ kind: "canvas" as const, entries: map.get("canvas") ?? [] }] : []),
  ];
}
