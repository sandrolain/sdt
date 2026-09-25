import type { TreeEntry } from "./api";

export type EntryKind =
  | "wiki"
  | "analysis"
  | "notes"
  | "tasks"
  | "plan"
  | "worklog"
  | "decision"
  | "questions"
  | "proposal"
  | "prompt"
  | "commands"
  | "research"
  | "architecture"
  | "mermaid"
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
  "decision",
  "questions",
  "architecture",
  "proposal",
  "prompt",
  "commands",
  "research",
  "mermaid",
  "other",
];

/** Human plural labels for every kind's tree section header. */
const KIND_LABELS: Record<EntryFilterKind, string> = {
  wiki: "Wiki",
  analysis: "Analyses",
  notes: "Notes",
  tasks: "Tasks",
  plan: "Plans",
  worklog: "Work log",
  decision: "Decisions",
  questions: "Questions",
  architecture: "Architecture",
  proposal: "Proposals",
  prompt: "Prompts",
  commands: "Commands",
  research: "Research",
  mermaid: "Diagrams",
  other: "Other",
  canvas: "Canvas",
};

/** Map a raw entry to a display kind. Canvas entries are treated as their own kind. */
export function entryKind(e: TreeEntry): EntryFilterKind {
  if (e.canvas) return "canvas";
  if (e.mermaid) return "mermaid";
  const k = e.kind ?? "other";
  return (KIND_ORDER as string[]).includes(k) ? (k as EntryKind) : "other";
}

export function kindLabel(k: EntryFilterKind): string {
  return KIND_LABELS[k] ?? KIND_LABELS.other;
}

/** Material Symbols glyph for a kind folder/entry. */
const KIND_ICONS: Record<EntryFilterKind, string> = {
  wiki: "menu_book",
  analysis: "analytics",
  notes: "sticky_note_2",
  tasks: "checklist",
  plan: "map",
  worklog: "history",
  decision: "gavel",
  questions: "help",
  proposal: "description",
  research: "travel_explore",
  prompt: "terminal",
  commands: "bolt",
  architecture: "account_tree",
  other: "description",
  canvas: "dashboard",
  mermaid: "polyline",
};

/** Catppuccin token (CSS var reference) for a kind. */
const KIND_COLORS: Record<EntryFilterKind, string> = {
  wiki: "var(--ctp-blue)",
  analysis: "var(--ctp-mauve)",
  notes: "var(--ctp-green)",
  tasks: "var(--ctp-yellow)",
  plan: "var(--ctp-peach)",
  worklog: "var(--ctp-overlay1)",
  decision: "var(--ctp-red)",
  questions: "var(--ctp-sky)",
  proposal: "var(--ctp-sapphire)",
  research: "var(--ctp-pink)",
  prompt: "var(--ctp-teal)",
  commands: "var(--ctp-maroon)",
  architecture: "var(--ctp-lavender)",
  other: "var(--ctp-overlay0)",
  canvas: "var(--ctp-teal)",
  mermaid: "var(--ctp-flamingo)",
};

export function kindIcon(k: EntryFilterKind): string {
  return KIND_ICONS[k] ?? KIND_ICONS.other;
}

export function kindColor(k: EntryFilterKind): string {
  return KIND_COLORS[k] ?? KIND_COLORS.other;
}

/** Corpus folder names whose kind name differs (plural or renamed). */
const FOLDER_KINDS: Record<string, EntryKind> = {
  plans: "plan",
  decisions: "decision",
  proposals: "proposal",
  prompts: "prompt",
  commands: "commands",
};

/** Best-effort kind from a corpus path alone (`context/<folder>/…`). */
export function kindFromPath(path: string): EntryFilterKind {
  if (path.endsWith(".mmd")) return "mermaid";
  const segments = path.split("/").filter(Boolean);
  if (segments.length < 3) return "other";
  const folder = segments[1];
  const kind = FOLDER_KINDS[folder] ?? folder;
  return (KIND_ORDER as string[]).includes(kind) ? (kind as EntryKind) : "other";
}

/** All kinds present in the tree, canvas represented specially, sorted by KIND_ORDER. */
export function availableKinds(entries: TreeEntry[]): EntryFilterKind[] {
  const seen = new Set<EntryFilterKind>();
  for (const e of entries) seen.add(entryKind(e));
  const kinds: EntryFilterKind[] = KIND_ORDER.filter((k) => seen.has(k));
  if (seen.has("canvas")) kinds.push("canvas");
  return kinds;
}
