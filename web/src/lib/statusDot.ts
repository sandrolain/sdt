import type { TreeEntry } from "./api";

export type StatusTone = "danger" | "warn" | "ok" | "neutral";
type ActiveStatusTone = Exclude<StatusTone, "neutral">;

export interface StatusDot {
  tone: StatusTone;
  /** tooltip explaining the tone */
  label: string;
}

export interface TaskProgress {
  tone: ActiveStatusTone;
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

/** Plans indexed by each analysis path referenced in their sources/links. */
export function plansByAnalysis(entries: TreeEntry[]): Map<string, TreeEntry[]> {
  const index = new Map<string, TreeEntry[]>();
  for (const entry of entries) {
    if (entry.kind !== "plan") continue;
    for (const ref of entry.sources ?? []) {
      const analysisPath = normalizeRef(ref);
      const plans = index.get(analysisPath);
      if (plans) {
        if (!plans.some((plan) => plan.path === entry.path)) plans.push(entry);
      } else {
        index.set(analysisPath, [entry]);
      }
    }
  }
  return index;
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

/** Objective a task inherits from the plan it references (its own `objective`
 *  wins when present, e.g. a standalone checklist). "" when unresolved. */
export function taskObjective(entry: TreeEntry, plans: Map<string, TreeEntry>): string {
  if (entry.objective) return entry.objective;
  if (entry.kind !== "tasks") return "";
  const planRef = (entry.sources ?? []).map(normalizeRef).find((p) => p.includes("/plan/")) ?? "";
  return (planRef && plans.get(planRef)?.objective) || "";
}

/** Copy of the task entries carrying their inherited objective, so
 *  groupByObjective can bucket them like analyses and plans. */
export function withTaskObjectives(tasks: TreeEntry[], plans: Map<string, TreeEntry>): TreeEntry[] {
  return tasks.map((task) => {
    if (task.objective) return task;
    const objective = taskObjective(task, plans);
    return objective ? { ...task, objective } : task;
  });
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

/** Status dot for plans, tasks, analyses and questions, including derived progress. */
export function statusDot(
  entry: TreeEntry,
  analysisPlans: Map<string, TreeEntry[]>,
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
    const status = (entry.status ?? "").trim().toLowerCase();
    if (isDoneStatus(status) || status === "resolved") {
      return { tone: "neutral", label: "Analysis archived" };
    }
    const plans = analysisPlans.get(entry.path) ?? [];
    if (plans.length === 0) return { tone: "danger", label: "Analysis without a plan" };
    const allPlanTasksDone = plans.every((plan) => {
      const tasks = taskIndex.get(normalizeRef(plan.path)) ?? [];
      return tasks.length > 0 && tasks.every((task) => isDoneStatus(task.status));
    });
    if (allPlanTasksDone) return { tone: "ok", label: "Analysis completed" };
    return { tone: "warn", label: "Analysis plan in progress" };
  }
  if (entry.kind === "questions") {
    const status = (entry.status ?? "").trim().toLowerCase();
    if (status === "active") return { tone: "danger", label: "Question unresolved" };
    if (status === "resolved") return { tone: "ok", label: "Question resolved" };
    return null;
  }
  return null;
}

/** Aggregate non-neutral analysis dots: none completed = red, some = yellow, all = green. */
export function groupDot(
  entries: TreeEntry[],
  analysisPlans: Map<string, TreeEntry[]>,
  taskIndex: Map<string, TreeEntry[]> = new Map(),
): StatusDot | null {
  const dots = entries
    .filter((entry) => entry.kind === "analysis")
    .map((entry) => statusDot(entry, analysisPlans, taskIndex))
    .filter((dot): dot is StatusDot => dot !== null && dot.tone !== "neutral");
  if (dots.length === 0) return null;

  const done = dots.filter((dot) => dot.tone === "ok").length;
  const tone: ActiveStatusTone = done === 0 ? "danger" : done === dots.length ? "ok" : "warn";
  const label = `${done}/${dots.length} analyses completed`;
  return { tone, label };
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
