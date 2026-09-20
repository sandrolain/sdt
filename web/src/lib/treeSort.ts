import type { TreeEntry } from "./api";
import { KIND_ORDER, entryKind, type EntryFilterKind } from "./kinds";
import { normalizeRef } from "./statusDot";
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

export interface ObjectiveGroup {
  /** frontmatter `objective` slug; "" collects entries without one */
  objective: string;
  entries: TreeEntry[];
}

/** Split analysis entries by their `objective` slug: named objectives come
 *  first in lexicographic order, then the "" group holding entries without an
 *  objective (they stay at the kind-folder root in the tree). */
export function groupByObjective(entries: TreeEntry[]): ObjectiveGroup[] {
  const map = new Map<string, TreeEntry[]>();
  for (const entry of entries) {
    const key = entry.objective ?? "";
    const bucket = map.get(key);
    if (bucket) bucket.push(entry);
    else map.set(key, [entry]);
  }
  const slugs = [...map.keys()].filter((slug) => slug !== "").sort();
  const groups: ObjectiveGroup[] = slugs.map((objective) => ({
    objective,
    entries: map.get(objective) ?? [],
  }));
  const ungrouped = map.get("");
  if (ungrouped) groups.push({ objective: "", entries: ungrouped });
  return groups;
}

export interface FolderGroup {
  /** folder segment name, e.g. `regulations` */
  name: string;
  /** path relative to the base prefix, e.g. `regulations/sub` */
  path: string;
  /** entries directly inside this folder */
  entries: TreeEntry[];
  children: FolderGroup[];
}

/** Split entries by the folder structure under `basePrefix`: entries directly
 *  in the base (or outside it) stay in `rootEntries`, deeper entries nest under
 *  `folders` (sorted by name, recursively). */
export function groupByFolder(
  entries: TreeEntry[],
  basePrefix: string,
): { rootEntries: TreeEntry[]; folders: FolderGroup[] } {
  const prefix = basePrefix.endsWith("/") ? basePrefix : `${basePrefix}/`;
  const rootEntries: TreeEntry[] = [];
  const folders: FolderGroup[] = [];
  const byPath = new Map<string, FolderGroup>();
  for (const entry of entries) {
    if (!entry.path.startsWith(prefix)) {
      rootEntries.push(entry);
      continue;
    }
    const segs = entry.path.slice(prefix.length).split("/").filter(Boolean);
    if (segs.length <= 1) {
      rootEntries.push(entry);
      continue;
    }
    let parentPath = "";
    let siblings = folders;
    let node: FolderGroup | undefined;
    for (const seg of segs.slice(0, -1)) {
      const path = parentPath ? `${parentPath}/${seg}` : seg;
      let existing = byPath.get(path);
      if (!existing) {
        existing = { name: seg, path, entries: [], children: [] };
        byPath.set(path, existing);
        siblings.push(existing);
      }
      node = existing;
      parentPath = path;
      siblings = existing.children;
    }
    node?.entries.push(entry);
  }
  const sortFolders = (nodes: FolderGroup[]) => {
    nodes.sort((a, b) => collator.compare(a.name, b.name));
    for (const n of nodes) sortFolders(n.children);
  };
  sortFolders(folders);
  return { rootEntries, folders };
}

/** Total entries in a folder subtree (direct + descendants), for count badges. */
export function folderCount(node: FolderGroup): number {
  return node.entries.length + node.children.reduce((sum, child) => sum + folderCount(child), 0);
}

export interface PlanGroup {
  /** normalized plan corpus path; "" collects tasks without a plan reference */
  plan: string;
  label: string;
  entries: TreeEntry[];
}

/** Group task entries under the plan they reference (`sources`/`links`).
 *  Named plan groups come first, ordered by plan `created` descending, then the
 *  "" group holding tasks without a plan reference (they stay at the root). */
export function groupByPlan(tasks: TreeEntry[], plans: Map<string, TreeEntry>): PlanGroup[] {
  const map = new Map<string, TreeEntry[]>();
  for (const task of tasks) {
    const ref = (task.sources ?? []).map(normalizeRef).find((p) => p.includes("/plan/")) ?? "";
    const bucket = map.get(ref);
    if (bucket) bucket.push(task);
    else map.set(ref, [task]);
  }
  const planPaths = [...map.keys()].filter((p) => p !== "");
  planPaths.sort((a, b) => {
    const ac = plans.get(a)?.created ?? "";
    const bc = plans.get(b)?.created ?? "";
    if (ac === bc) return collator.compare(a, b);
    if (ac === "") return 1;
    if (bc === "") return -1;
    return collator.compare(bc, ac);
  });
  const groups: PlanGroup[] = planPaths.map((plan) => ({
    plan,
    label: planLabel(plan, plans.get(plan)),
    entries: map.get(plan) ?? [],
  }));
  const ungrouped = map.get("");
  if (ungrouped) groups.push({ plan: "", label: "", entries: ungrouped });
  return groups;
}

/** Plan group label: the plan title, else the slug from its filename with the
 *  date prefix and `.md` removed. */
function planLabel(path: string, plan?: TreeEntry): string {
  if (plan) return displayTitle({ title: plan.title, path: plan.path });
  const base = path.split("/").pop() ?? path;
  return base.replace(/\.md$/, "").replace(/^\d{8}-(\d{6}-)?/, "");
}
