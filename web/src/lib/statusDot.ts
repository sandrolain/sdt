import type { TreeEntry } from "./api";

export type StatusTone = "danger" | "warn" | "ok";

export interface StatusDot {
  tone: StatusTone;
  /** tooltip explaining the tone */
  label: string;
}

const IN_PROGRESS = new Set(["in-progress", "in_progress", "wip", "progress", "doing"]);
const DONE = new Set(["completed", "complete", "done", "executed", "archived"]);

/** True when a frontmatter `status` value is a "done" state (completed/archived). */
export function isDoneStatus(status?: string): boolean {
  if (!status) return false;
  return DONE.has(status.trim().toLowerCase());
}

/** Corpus-relative path of a frontmatter reference, normalised to `context/...md`. */
function normalizeRef(ref: string): string {
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

/**
 * Status dot for a tree entry: plans and tasks report execution state, an
 * analysis not referenced by any plan reports "no plan yet"; other kinds have
 * no dot.
 */
export function statusDot(entry: TreeEntry, plannedAnalyses: Set<string>): StatusDot | null {
  if (isDoneStatus(entry.status)) return null;
  const status = (entry.status ?? "").trim().toLowerCase();
  if (entry.kind === "plan" || entry.kind === "tasks") {
    if (status === "") return null; // unknown state → no indicator
    if (isDoneStatus(status)) {
      return entry.kind === "plan"
        ? { tone: "ok", label: "Plan executed" }
        : { tone: "ok", label: "Task executed" };
    }
    if (entry.kind === "tasks" && IN_PROGRESS.has(status)) {
      return { tone: "warn", label: "Task in progress" };
    }
    return entry.kind === "plan"
      ? { tone: "danger", label: "Plan not executed" }
      : { tone: "danger", label: "Task not executed" };
  }
  if (entry.kind === "analysis") {
    return plannedAnalyses.has(entry.path)
      ? null
      : { tone: "warn", label: "Analysis without a plan" };
  }
  return null;
}
