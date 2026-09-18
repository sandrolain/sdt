import type { TreeEntry } from "./api";
import { KIND_ORDER, entryKind, type EntryFilterKind } from "./kinds";
import { displayTitle } from "./titles";

export type TreeSortKey =
  | "name_asc"
  | "name_desc"
  | "title_asc"
  | "title_desc"
  | "created_asc"
  | "created_desc"
  | "modified_asc"
  | "modified_desc";

export const TREE_SORTS: { id: TreeSortKey; label: string }[] = [
  { id: "name_asc", label: "Name ASC" },
  { id: "name_desc", label: "Name DESC" },
  { id: "title_asc", label: "Title ASC" },
  { id: "title_desc", label: "Title DESC" },
  { id: "created_asc", label: "Created ASC" },
  { id: "created_desc", label: "Created DESC" },
  { id: "modified_asc", label: "Modified ASC" },
  { id: "modified_desc", label: "Modified DESC" },
];

export interface TreeGroup {
  kind: EntryFilterKind;
  entries: TreeEntry[];
}

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

function sortValue(entry: TreeEntry, key: TreeSortKey): string {
  switch (key) {
    case "name_asc":
    case "name_desc":
      return entry.path.split("/").filter(Boolean).pop() ?? entry.path;
    case "title_asc":
    case "title_desc":
      return displayTitle({ title: entry.title, path: entry.path });
    case "created_asc":
    case "created_desc":
      return entry.created ?? "";
    case "modified_asc":
    case "modified_desc":
      return entry.modified ?? "";
  }
}

/** Sort entries with locale-aware collation; dates compare ISO-lexicographically.
 *  Entries missing the sort field stay last in both directions. */
export function sortEntries(entries: TreeEntry[], key: TreeSortKey): TreeEntry[] {
  const dir = key.endsWith("_asc") ? "asc" : "desc";
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
