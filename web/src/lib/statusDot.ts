import type { TreeEntry } from "./api";
import { STATUS_VALUES, valueLabel } from "./frontmatter";

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
  | "investigating"
  | "blocked"
  | "deferred"
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
  "investigating",
  "blocked",
  "deferred",
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
  investigating: { label: "Investigating", family: "open" },
  blocked: { label: "Blocked", family: "open" },
  deferred: { label: "Deferred", family: "deferred" },
  accepted: { label: "Accepted", family: "concluded" },
  postponed: { label: "Postponed", family: "deferred" },
  rejected: { label: "Rejected", family: "retired" },
  abandoned: { label: "Abandoned", family: "retired" },
  archived: { label: "Archived", family: "retired" },
  deprecated: { label: "Deprecated", family: "retired" },
  superseded: { label: "Superseded", family: "retired" },
  "no-state": { label: "No state", family: "unclassified" },
};

/** One-click family presets for the tree state filter: `all` plus the broad
 *  lifecycle buckets. `closed` is the concluded + retired union; `no-state`
 *  (unclassified) belongs to `all` only. */
export type StatePresetId = "all" | "open" | "closed" | "deferred";

export interface StatePreset {
  id: StatePresetId;
  label: string;
  keys: StateKey[];
}

export const STATE_PRESETS: readonly StatePreset[] = [
  { id: "all", label: "All", keys: [...STATE_KEYS] },
  {
    id: "open",
    label: "Open",
    keys: STATE_KEYS.filter((key) => stateMeta[key].family === "open"),
  },
  {
    id: "closed",
    label: "Closed",
    keys: STATE_KEYS.filter(
      (key) => stateMeta[key].family === "concluded" || stateMeta[key].family === "retired",
    ),
  },
  {
    id: "deferred",
    label: "Deferred",
    keys: STATE_KEYS.filter((key) => stateMeta[key].family === "deferred"),
  },
];

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
  /** why the declared status and the effective state disagree, with the
   *  remedy, when they do. Mirrors the CLI drift lint (see `driftReasons`). */
  drift?: string;
}

const SYNC = "run `sdt context sync`";

/** The statuses the CLI reconciler is allowed to advance
 *  (`statusFlappable` in cli/cmd/context_cascade.go). A disagreement on a
 *  user-owned status is not drift: `archived`, `draft`, `postponed` and
 *  `abandoned` are the user's own decisions. */
const FLAPPABLE = new Set(["active", "completed"]);

/** The per-kind status vocabulary, read from the single transcription that the
 *  drift guard in frontmatter.test.ts ties to the status matrix. A kind with no
 *  entry (worklog, notes, tmp) carries no status at all. */
function statusVocabulary(kind?: string): string[] {
  const prefix = `${kind ?? ""}.`;
  return Object.keys(STATUS_VALUES)
    .filter((key) => key.startsWith(prefix))
    .map((key) => key.slice(prefix.length));
}

/** Declared-vs-derived drift, mirroring `lintCascadeDrift` and
 *  `lintPlanTaskAgreement` in cli/cmd for the edges the tree payload carries
 *  (plan↔task files, analysis↔plans, and the per-kind vocabulary).
 *
 *  One rule is deliberately not mirrored: `task declares completed but its
 *  checklist has unfinished items`. It needs the task file's own checklist
 *  items, which the tree payload does not carry (viewer/server.go), so it
 *  stays a CLI lint warning — do not "fix" it by parsing markdown in the
 *  browser. */
function driftReasons(
  entry: TreeEntry,
  analysisPlans: Map<string, TreeEntry[]>,
  taskIndex: Map<string, TreeEntry[]>,
): string {
  const declared = (entry.status ?? "").trim().toLowerCase();
  const kind = entry.kind;

  if (
    kind &&
    statusVocabulary(kind).length > 0 &&
    declared !== "" &&
    !statusVocabulary(kind).includes(declared)
  ) {
    return `declared status \`${declared}\` is outside the ${kind} vocabulary (${statusVocabulary(kind).join(" | ")})`;
  }

  if (kind === "plan") {
    const tasks = taskIndex.get(normalizeRef(entry.path)) ?? [];
    if (declared !== "completed") return "";
    if (tasks.length === 0)
      return "plan declares completed but no task file references it (cannot verify completion)";
    const unfinished = tasks
      .filter((t) => !isDoneStatus(t.status))
      .map((t) => t.path.split("/").pop());
    if (unfinished.length > 0) {
      return `plan declares completed but ${unfinished.length} task(s) are not done: ${unfinished.join(", ")}`;
    }
    return "";
  }

  if (kind === "analysis") {
    const plans = analysisPlans.get(entry.path) ?? [];
    if (declared === "completed") {
      const unfinished = plans
        .filter((plan) => !isPlanDone(plan, taskIndex))
        .map((plan) => plan.path.split("/").pop());
      if (unfinished.length > 0) {
        return `analysis declares completed but ${unfinished.length} plan(s) are not done: ${unfinished.join(", ")}`;
      }
      return "";
    }
    if (
      FLAPPABLE.has(declared) &&
      plans.length > 0 &&
      plans.every((plan) => isPlanDone(plan, taskIndex))
    ) {
      return `analysis is derivably completed (all plans done) — ${SYNC}`;
    }
  }

  return "";
}

/** A plan is done when it declares a terminal status or when every task file
 *  it references is done — the two inputs `derivePlanStatus` combines on the
 *  CLI side. */
function isPlanDone(plan: TreeEntry, taskIndex: Map<string, TreeEntry[]>): boolean {
  const declared = (plan.status ?? "").trim().toLowerCase();
  if (PLAN_TERMINAL.has(declared)) return true;
  const tasks = taskIndex.get(normalizeRef(plan.path)) ?? [];
  return tasks.length > 0 && tasks.every((t) => isDoneStatus(t.status));
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

/** The plans an analysis is the parent of, from the server-resolved `plans`
 *  field. An analysis with no plan resolves to an empty list, which is the
 *  honest reading when its work was delivered by a sibling analysis' plan. */
export function plansByAnalysis(entries: TreeEntry[]): Map<string, TreeEntry[]> {
  const index = new Map<string, TreeEntry[]>();
  const byPath = new Map(entries.map((entry) => [entry.path, entry]));
  for (const entry of entries) {
    if (entry.kind !== "analysis") continue;
    const plans = (entry.plans ?? [])
      .map((path) => byPath.get(path))
      .filter((plan): plan is TreeEntry => plan !== undefined);
    if (plans.length > 0) index.set(entry.path, plans);
  }
  return index;
}

/** Task entries indexed by their resolved plan (`plan` field, "" = no plan, so
 *  they stay at the tree root). The edge is the typed relation the server
 *  resolved; a `sources` citation never puts a task file in a plan group. */
export function tasksByPlan(entries: TreeEntry[]): Map<string, TreeEntry[]> {
  const map = new Map<string, TreeEntry[]>();
  for (const entry of entries) {
    if (entry.kind !== "tasks") continue;
    const planRef = entry.plan ?? "";
    const bucket = map.get(planRef);
    if (bucket) bucket.push(entry);
    else map.set(planRef, [entry]);
  }
  return map;
}

/** Objective a task inherits from the plan it derives from (its own
 *  `objective` wins when present, e.g. a standalone checklist). "" when
 *  unresolved. The plan is the server-resolved `plan` field. */
export function taskObjective(entry: TreeEntry, plans: Map<string, TreeEntry>): string {
  if (entry.objective) return entry.objective;
  if (entry.kind !== "tasks") return "";
  const planRef = entry.plan ?? "";
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
  const state = rawState(entry, analysisPlans, taskIndex);
  const drift = driftReasons(entry, analysisPlans, taskIndex);
  return drift ? { ...state, drift } : state;
}

/** The state itself, before the declared-vs-derived check. */
function rawState(
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
    // the same notion of "plan done" the drift rule and the CLI reconciler use,
    // so the effective state and the drift message can never disagree
    if (plans.every((plan) => isPlanDone(plan, taskIndex))) {
      return declared === ""
        ? { key: "completed", tone: "ok", label: "Analysis completed" }
        : { key: "completed", tone: "ok", label: "Analysis completed", declared };
    }
    return { key: "in-progress", tone: "warn", label: "Analysis plan in progress" };
  }

  if (entry.kind === "questions") {
    switch (declared) {
      case "active":
        return { key: "active", tone: "danger", label: "Question unresolved" };
      case "investigating":
        return { key: "investigating", tone: "warn", label: "Question under investigation" };
      case "blocked":
        return { key: "blocked", tone: "danger", label: "Question blocked" };
      case "deferred":
        return { key: "deferred", tone: "neutral", label: "Question deferred" };
      case "resolved":
        return { key: "resolved", tone: "ok", label: "Question resolved" };
    }
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
