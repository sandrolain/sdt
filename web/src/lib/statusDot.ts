import type { TreeEntry } from "./api";
import { valueLabel } from "./frontmatter";

export type StatusTone = "danger" | "warn" | "ok" | "neutral" | "draft";
type ActiveStatusTone = Exclude<StatusTone, "neutral" | "draft">;

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

/** Effective state of a document, kind-agnostic and closed: the one model
 *  behind the entry dot, the group aggregate dots, the metadata-panel status
 *  and the tree state filter. A derived state (a plan's progress, an analysis
 *  whose plans are all done) can differ from the declared `status`, which is
 *  why the two are reported separately. */
export type StateKey =
  | "draft"
  | "active"
  | "proposed"
  | "review"
  | "current"
  | "not-started"
  | "pending"
  | "in-progress"
  | "not-executed"
  | "no-plan"
  | "completed"
  | "resolved"
  | "accepted"
  | "postponed"
  | "rejected"
  | "abandoned"
  | "archived"
  | "deprecated"
  | "superseded"
  | "no-state";

/** Lifecycle family of a state, used to group the filter's options. */
export type StateFamily = "open" | "concluded" | "deferred" | "retired" | "unclassified";

export interface StateMeta {
  label: string;
  family: StateFamily;
}

/** Every state `entryState` can emit, in lifecycle order. The order and the
 *  families are presentation, so the list is written by hand; a guard test
 *  asserts the model emits exactly this set, so a state cannot exist in one
 *  place only. */
export const STATE_KEYS: readonly StateKey[] = [
  "draft",
  "active",
  "proposed",
  "review",
  "current",
  "not-started",
  "pending",
  "in-progress",
  "not-executed",
  "no-plan",
  "completed",
  "resolved",
  "accepted",
  "postponed",
  "rejected",
  "abandoned",
  "archived",
  "deprecated",
  "superseded",
  "no-state",
];

/** Option label and family per state. No colour swatch: `draft` is peach for an
 *  analysis and warn for a proposal, so a per-state tone would be a lie — the
 *  dot keeps its per-kind tone. */
export const stateMeta: Record<StateKey, StateMeta> = {
  draft: { label: "Draft", family: "open" },
  active: { label: "Active", family: "open" },
  proposed: { label: "Proposed", family: "open" },
  review: { label: "Review", family: "open" },
  current: { label: "Current", family: "open" },
  "not-started": { label: "Not started", family: "open" },
  pending: { label: "Pending", family: "open" },
  "in-progress": { label: "In progress", family: "open" },
  "not-executed": { label: "Not executed", family: "open" },
  "no-plan": { label: "No plan", family: "open" },
  completed: { label: "Completed", family: "concluded" },
  resolved: { label: "Resolved", family: "concluded" },
  accepted: { label: "Accepted", family: "concluded" },
  postponed: { label: "Postponed", family: "deferred" },
  rejected: { label: "Rejected", family: "retired" },
  abandoned: { label: "Abandoned", family: "retired" },
  archived: { label: "Archived", family: "retired" },
  deprecated: { label: "Deprecated", family: "retired" },
  superseded: { label: "Superseded", family: "retired" },
  "no-state": { label: "No state", family: "unclassified" },
};

/** The kinds whose state is a lifecycle dot. The other kinds have a state (the
 *  filter needs it) but no dot, exactly as before the state model. */
const DOT_KINDS = new Set(["plan", "tasks", "analysis", "questions"]);

/** The full state of a tree entry: the key the filter selects on, the dot tone
 *  and tooltip, and the declared status when the effective state overrode or
 *  ignored it. */
export interface EntryState {
  key: StateKey;
  tone: StatusTone;
  label: string;
  declared?: string;
}

const IN_PROGRESS = new Set(["in-progress", "in_progress", "wip", "progress", "doing"]);
const DONE = new Set(["completed", "complete", "done", "executed", "archived"]);

/** A plan's terminal declared statuses: they decide the dot over the task set. */
const PLAN_TERMINAL = new Set(["completed", "abandoned"]);

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

/** Analysis paths a plan derives from, i.e. its frontmatter `sources` list.
 *  `links` is generic correlation and never reaches a derived status. */
export function planReferencedAnalyses(entries: TreeEntry[]): Set<string> {
  const referenced = new Set<string>();
  for (const entry of entries) {
    if (entry.kind !== "plan") continue;
    for (const ref of entry.sources ?? []) referenced.add(normalizeRef(ref));
  }
  return referenced;
}

/** Plans indexed by each analysis path in their `sources` list (the derivation
 *  edge). A plan that only `links` an analysis is not indexed under it, so a
 *  still-open analysis keeps reading "Analysis without a plan"; an archived one
 *  is resolved earlier and never reaches this map. */
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

/** Task entries indexed by the normalized plan path in their `sources` list
 *  ("" = no sourced plan, so they stay at the tree root). */
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

/** Objective a task inherits from the plan it sources (its own `objective`
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

/** Effective state of a tree entry: the declared `status` mapped through the
 *  per-kind vocabulary, refined by the lifecycle chain for the four derived
 *  kinds (plan from its task files, analysis from its plans, task from its own
 *  status, question from its resolution). Tones and labels are the ones the dot
 *  has always shown. */
export function entryState(
  entry: TreeEntry,
  analysisPlans: Map<string, TreeEntry[]> = new Map(),
  taskIndex: Map<string, TreeEntry[]> = new Map(),
): EntryState {
  const declared = (entry.status ?? "").trim().toLowerCase();

  if (entry.kind === "plan") {
    // A terminal declared status (completed/abandoned) decides the state over
    // the task set (D4); only a non-terminal plan derives from it.
    if (PLAN_TERMINAL.has(declared)) {
      const value = valueLabel("plan", declared);
      return {
        key: declared === "completed" ? "completed" : "abandoned",
        tone: value?.tone ?? "neutral",
        label: `Plan ${(value?.label ?? declared).toLowerCase()}`,
      };
    }
    const tasks = taskIndex.get(normalizeRef(entry.path)) ?? [];
    const progress = taskProgress(tasks);
    const started =
      progress.done > 0 ||
      tasks.some((t) => IN_PROGRESS.has((t.status ?? "").trim().toLowerCase()));
    const key: StateKey =
      progress.tone === "ok"
        ? "completed"
        : tasks.length === 0
          ? "not-started"
          : started
            ? "in-progress"
            : "not-started";
    const label =
      progress.tone === "ok"
        ? "Plan completed"
        : progress.tone === "warn"
          ? "Plan in progress"
          : progress.total === 0
            ? "Plan not started"
            : taskProgressLabel(progress);
    // A plan whose tasks are all done while it still declares `active` is the
    // declared-vs-derived disagreement the drift warning reports.
    return key === "completed" && declared !== ""
      ? { key, tone: progress.tone, label, declared }
      : { key, tone: progress.tone, label };
  }

  if (entry.kind === "tasks") {
    if (isDoneStatus(declared)) return { key: "completed", tone: "ok", label: "Task completed" };
    if (IN_PROGRESS.has(declared)) {
      return { key: "in-progress", tone: "warn", label: "Task in progress" };
    }
    if (declared === "") return { key: "pending", tone: "danger", label: "Task not started" };
    return { key: "not-executed", tone: "danger", label: "Task not executed" };
  }

  if (entry.kind === "analysis") {
    if (declared === "completed")
      return { key: "completed", tone: "ok", label: "Analysis completed" };
    // An out-of-vocabulary value (e.g. the legacy `resolved`) reads as archived
    // and keeps the raw value as `declared`, so the vocabulary drift is visible.
    if (isDoneStatus(declared) || declared === "resolved") {
      return { key: "archived", tone: "neutral", label: "Analysis archived", declared };
    }
    if (declared === "draft")
      return { key: "draft", tone: "draft", label: "Analysis to be written" };
    // Concluded but deliberately deferred: not "in progress", and not a
    // derivation over a plan set. A neutral state keeps it out of the
    // completed/total aggregate (groupDot drops neutral and draft tones).
    if (declared === "postponed") {
      return { key: "postponed", tone: "neutral", label: "Analysis postponed" };
    }
    const plans = analysisPlans.get(entry.path) ?? [];
    if (plans.length === 0)
      return { key: "no-plan", tone: "danger", label: "Analysis without a plan" };
    const allPlanTasksDone = plans.every((plan) => {
      const tasks = taskIndex.get(normalizeRef(plan.path)) ?? [];
      return tasks.length > 0 && tasks.every((task) => isDoneStatus(task.status));
    });
    if (allPlanTasksDone) {
      return declared === ""
        ? { key: "completed", tone: "ok", label: "Analysis completed" }
        : { key: "completed", tone: "ok", label: "Analysis completed", declared };
    }
    return { key: "in-progress", tone: "warn", label: "Analysis plan in progress" };
  }

  if (entry.kind === "questions") {
    if (declared === "active")
      return { key: "active", tone: "danger", label: "Question unresolved" };
    if (declared === "resolved") return { key: "resolved", tone: "ok", label: "Question resolved" };
    return {
      key: "no-state",
      tone: "neutral",
      label: "No state",
      ...(declared ? { declared } : {}),
    };
  }

  // The remaining kinds have no derived state: their declared status maps
  // straight through the shared vocabulary transcription.
  if (declared === "") return { key: "no-state", tone: "neutral", label: "No state" };
  const value = valueLabel(entry.kind ?? "other", declared);
  if (!value) return { key: "no-state", tone: "neutral", label: "No state", declared };
  return { key: declared as StateKey, tone: value.tone, label: value.label };
}

/** Status dot for plans, tasks, analyses and questions, including derived
 *  progress: the renderer over `entryState`, null for the kinds without a dot
 *  and for a state that carries none. */
export function statusDot(
  entry: TreeEntry,
  analysisPlans: Map<string, TreeEntry[]> = new Map(),
  taskIndex: Map<string, TreeEntry[]> = new Map(),
): StatusDot | null {
  if (!DOT_KINDS.has(entry.kind ?? "")) return null;
  const state = entryState(entry, analysisPlans, taskIndex);
  if (state.key === "no-state") return null;
  return { tone: state.tone, label: state.label };
}

/** Aggregate non-neutral analysis dots: none completed = red, some = yellow, all = green. */
export function groupDot(
  entries: TreeEntry[],
  analysisPlans: Map<string, TreeEntry[]>,
  taskIndex: Map<string, TreeEntry[]> = new Map(),
): StatusDot | null {
  const states = entries
    .filter((entry) => entry.kind === "analysis")
    .map((entry) => entryState(entry, analysisPlans, taskIndex))
    .filter((state) => state.tone !== "neutral" && state.tone !== "draft");
  if (states.length === 0) return null;

  const done = states.filter((state) => state.tone === "ok").length;
  const tone: ActiveStatusTone = done === 0 ? "danger" : done === states.length ? "ok" : "warn";
  const label = `${done}/${states.length} analyses completed`;
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
    // A terminal declared status (completed/abandoned, plus any archived
    // spelling) counts the plan as completed regardless of its referenced tasks
    // (D4, aligning the hide filter with the dot model).
    const declared = (entry.status ?? "").trim().toLowerCase();
    if (isDoneStatus(declared) || PLAN_TERMINAL.has(declared)) return true;
    const tasks = taskIndex.get(normalizeRef(entry.path)) ?? [];
    return tasks.length > 0 && tasks.every((t) => isDoneStatus(t.status));
  }
  return false;
}
