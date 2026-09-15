import type { TreeEntry } from "./api";

export type EntryKind =
  | "wiki"
  | "analysis"
  | "notes"
  | "tasks"
  | "plan"
  | "worklog"
  | "decisions"
  | "questions"
  | "rfc"
  | "prompts"
  | "architecture"
  | "other";
export type EntryFilterKind = EntryKind | "canvas";

/** Stable, deduped list of visible kinds, in corpus-sensible order. */
export const KIND_ORDER: EntryKind[] = [
  "wiki",
  "analysis",
  "notes",
  "plan",
  "tasks",
  "worklog",
  "decisions",
  "questions",
  "architecture",
  "other",
];

/** Map a raw entry to a display kind. Canvas entries are treated as their own kind. */
export function entryKind(e: TreeEntry): EntryFilterKind {
  if (e.canvas) return "canvas";
  const k = e.kind ?? "other";
  return (KIND_ORDER as string[]).includes(k) ? (k as EntryKind) : "other";
}

export function kindLabel(k: EntryFilterKind): string {
  // Corpus kind names read naturally as labels; no pluralization needed.
  return k;
}

/** Material Symbols glyph for a kind folder/entry. */
const KIND_ICONS: Record<EntryFilterKind, string> = {
  wiki: "menu_book",
  analysis: "analytics",
  notes: "sticky_note_2",
  tasks: "checklist",
  plan: "map",
  worklog: "history",
  decisions: "gavel",
  questions: "help",
  rfc: "description",
  prompts: "terminal",
  architecture: "account_tree",
  other: "description",
  canvas: "dashboard",
};

/** Catppuccin token (CSS var reference) for a kind. */
const KIND_COLORS: Record<EntryFilterKind, string> = {
  wiki: "var(--ctp-blue)",
  analysis: "var(--ctp-mauve)",
  notes: "var(--ctp-green)",
  tasks: "var(--ctp-yellow)",
  plan: "var(--ctp-peach)",
  worklog: "var(--ctp-overlay1)",
  decisions: "var(--ctp-red)",
  questions: "var(--ctp-sky)",
  rfc: "var(--ctp-sapphire)",
  prompts: "var(--ctp-teal)",
  architecture: "var(--ctp-lavender)",
  other: "var(--ctp-overlay0)",
  canvas: "var(--ctp-teal)",
};

export function kindIcon(k: EntryFilterKind): string {
  return KIND_ICONS[k] ?? KIND_ICONS.other;
}

export function kindColor(k: EntryFilterKind): string {
  return KIND_COLORS[k] ?? KIND_COLORS.other;
}

/** All kinds present in the tree, canvas represented specially, sorted by KIND_ORDER. */
export function availableKinds(entries: TreeEntry[]): EntryFilterKind[] {
  const seen = new Set<EntryFilterKind>();
  for (const e of entries) seen.add(entryKind(e));
  const kinds: EntryFilterKind[] = KIND_ORDER.filter((k) => seen.has(k));
  if (seen.has("canvas")) kinds.push("canvas");
  return kinds;
}
