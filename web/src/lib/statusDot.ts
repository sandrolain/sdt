import type { TreeEntry } from "./api";

export type StatusTone = "danger" | "warn" | "ok";

export interface StatusDot {
  tone: StatusTone;
  /** tooltip explaining the tone */
  label: string;
}

export interface TaskProgress {
  tone: StatusTone;
  done: number;
  total: number;
}

const IN_PROGRESS = new Set(["in-progress", "in_progress", "wip", "progress", "doing"]);
const DONE = new Set(["completed", "complete", "done", "executed", "archived"]);

/** True when a frontmatter `status` value is a "done" state (completed/archived). */
export function isDoneStatus(status?: string): boolean {
  if (!status) return false;
  return DONE.has(status.trim().toLowerCase());
}

/** Corpus-relative path of a frontmatter reference, normalised to `context/...md`. */
export function normalizeRef(ref: string): string {
  const clean = ref.trim().replace(/^\.\//, "").replace(/^\/+/, "");
  const withExt = clean.endsWith(".md") ? clean : `${clean}.md`;
  return withExt.startsWith("context/") ? withExt : `context/${withExt}`;
}

/** Analysis paths referenced by a plan (frontmatter `sources`/`links`). */
export function planReferencedAnalyses(entries: TreeEntry[]): Set<string> {
  const referenced = new Set<string>();
  for (const entry of entries) {
    if (entry.kind !== "plan") continue;
    for (const ref of entry.sources ?? []) referenced.add(normalizeRef(ref));
  }
  return referenced;
}

/** Task entries indexed by the normalized plan path they reference ("" = none). */
export function tasksByPlan(entries: TreeEntry[]): Map<string, TreeEntry[]> {
  const map = new Map<string, TreeEntry[]>();
  for (const entry of entries) {
    if (entry.kind !== "tasks") continue;
    const planRef = (entry.sources ?? []).map(normalizeRef).find((p) => p.includes("/plan/")) ?? "";
    const bucket = map.get(planRef);
    if (bucket) bucket.push(entry);
    else map.set(planRef, [entry]);
  }
  return map;
}

/** Progress of a task list: red none done (or no tasks), green all done, else yellow. */
export function taskProgress(tasks: TreeEntry[]): TaskProgress {
  const list = tasks.filter((t) => t.kind === "tasks");
  const done = list.filter((t) => isDoneStatus(t.status)).length;
  const total = list.length;
  let tone: StatusTone;
  if (total === 0 || done === 0) tone = "danger";
  else if (done === total) tone = "ok";
  else tone = "warn";
  return { tone, done, total };
}

/** Human label for a task-progress tone. */
export function taskProgressLabel(progress: TaskProgress): string {
  if (progress.tone === "ok") return "All tasks completed";
  if (progress.tone === "warn") return `${progress.done}/${progress.total} tasks completed`;
  return progress.total === 0 ? "No tasks started" : "No tasks completed yet";
}

/**
 * Status dot for a tree entry: plans report the progress of their referenced
 * tasks; tasks report their own execution state; an analysis not referenced by
 * any plan (and not done) reports "no plan yet"; other kinds have no dot.
 */
export function statusDot(
  entry: TreeEntry,
  plannedAnalyses: Set<string>,
  taskIndex: Map<string, TreeEntry[]> = new Map(),
): StatusDot | null {
  if (entry.kind === "plan") {
    const tasks = taskIndex.get(normalizeRef(entry.path)) ?? [];
    const { tone } = taskProgress(tasks);
    const label =
      tone === "ok" ? "Plan completed" : tone === "warn" ? "Plan in progress" : "Plan not started";
    return { tone, label };
  }
  if (entry.kind === "tasks") {
    const status = (entry.status ?? "").trim().toLowerCase();
    if (isDoneStatus(status)) return { tone: "ok", label: "Task completed" };
    if (IN_PROGRESS.has(status)) return { tone: "warn", label: "Task in progress" };
    if (status === "") return { tone: "danger", label: "Task not started" };
    return { tone: "danger", label: "Task not executed" };
  }
  if (entry.kind === "analysis") {
    if (isDoneStatus(entry.status)) return null;
    if (plannedAnalyses.has(entry.path)) return null;
    return { tone: "warn", label: "Analysis without a plan" };
  }
  return null;
}

/** True when the entry counts as completed for the hide filter. A plan is
 *  completed only through its referenced tasks (all done); a task/analysis
 *  only through a done `status`. Entries without tasks/status stay visible. */
export function entryCompleted(
  entry: TreeEntry,
  taskIndex: Map<string, TreeEntry[]> = new Map(),
): boolean {
  if (entry.kind === "tasks") return isDoneStatus(entry.status);
  if (entry.kind === "analysis") return isDoneStatus(entry.status);
  if (entry.kind === "plan") {
    const tasks = taskIndex.get(normalizeRef(entry.path)) ?? [];
    return tasks.length > 0 && tasks.every((t) => isDoneStatus(t.status));
  }
  return false;
}
