// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { TreeEntry } from "./api";
import {
  entryCompleted,
  groupDot,
  isDoneStatus,
  planReferencedAnalyses,
  plansByAnalysis,
  statusDot,
  taskObjective,
  taskProgress,
  tasksByPlan,
  withTaskObjectives,
} from "./statusDot";

function entry(patch: Partial<TreeEntry>): TreeEntry {
  return { path: "context/plan/x.md", kind: "plan", ...patch };
}

const plan = (path = "context/plan/x.md") => entry({ path, kind: "plan" });
const task = (path = "context/tasks/t.md", status?: string, sources?: string[]) =>
  entry({ path, kind: "tasks", status, sources });

describe("statusDot", () => {
  it("plans report the progress of their referenced tasks (red none done / yellow partial / green all done)", () => {
    const empty = new Map<string, TreeEntry[]>();
    expect(statusDot(plan(), new Map(), empty)).toEqual({
      tone: "danger",
      label: "Plan not started",
    });

    const noneDone = new Map<string, TreeEntry[]>([
      [
        "context/plan/x.md",
        [task("context/tasks/a.md", "pending"), task("context/tasks/b.md", "wip")],
      ],
    ]);
    expect(statusDot(plan(), new Map(), noneDone)?.tone).toBe("danger");

    const partial = new Map<string, TreeEntry[]>([
      [
        "context/plan/x.md",
        [task("context/tasks/a.md", "completed"), task("context/tasks/b.md", "pending")],
      ],
    ]);
    const partialDot = statusDot(plan(), new Map(), partial);
    expect(partialDot).toEqual({ tone: "warn", label: "Plan in progress" });

    const allDone = new Map<string, TreeEntry[]>([
      [
        "context/plan/x.md",
        [task("context/tasks/a.md", "completed"), task("context/tasks/b.md", "archived")],
      ],
    ]);
    expect(statusDot(plan(), new Map(), allDone)).toEqual({
      tone: "ok",
      label: "Plan completed",
    });
  });

  it("marks tasks by execution state, empty status red", () => {
    const t = (status?: string) => task("context/tasks/t.md", status);
    expect(statusDot(t("completed"), new Map())).toEqual({ tone: "ok", label: "Task completed" });
    expect(statusDot(t("in-progress"), new Map())).toEqual({
      tone: "warn",
      label: "Task in progress",
    });
    expect(statusDot(t(""), new Map())).toEqual({ tone: "danger", label: "Task not started" });
    expect(statusDot(t(undefined), new Map())?.tone).toBe("danger");
    expect(statusDot(t("active"), new Map())?.tone).toBe("danger");
  });

  it("maps analysis state to no plan, plan progress and completion", () => {
    const analysis = entry({ path: "context/analysis/a.md", kind: "analysis" });
    const planForAnalysis = plan("context/plan/a.md");
    planForAnalysis.sources = ["analysis/a.md"];
    const analysisPlans = plansByAnalysis([analysis, planForAnalysis]);

    expect(statusDot(analysis, new Map())).toEqual({
      tone: "danger",
      label: "Analysis without a plan",
    });
    expect(statusDot(analysis, analysisPlans)).toEqual({
      tone: "warn",
      label: "Analysis plan in progress",
    });

    const taskIndex = new Map<string, TreeEntry[]>([
      ["context/plan/a.md", [task("context/tasks/a.md", "pending")]],
    ]);
    expect(statusDot(analysis, analysisPlans, taskIndex)?.tone).toBe("warn");
    taskIndex.set("context/plan/a.md", [task("context/tasks/a.md", "completed")]);
    expect(statusDot(analysis, analysisPlans, taskIndex)).toEqual({
      tone: "ok",
      label: "Analysis completed",
    });
  });

  it("requires every referenced plan to have a non-empty completed task list", () => {
    const analysis = entry({ path: "context/analysis/a.md", kind: "analysis" });
    const planA = plan("context/plan/a.md");
    planA.sources = ["analysis/a.md"];
    const planB = plan("context/plan/b.md");
    planB.sources = ["analysis/a.md"];
    const plans = plansByAnalysis([analysis, planA, planB]);
    const tasks = new Map<string, TreeEntry[]>([
      ["context/plan/a.md", [task("context/tasks/a.md", "completed")]],
    ]);
    expect(statusDot(analysis, plans, tasks)?.tone).toBe("warn");
    tasks.set("context/plan/b.md", [task("context/tasks/b.md", "completed")]);
    expect(statusDot(analysis, plans, tasks)?.tone).toBe("ok");
  });

  it("gives done analyses a visible neutral tone, including resolved corpus values", () => {
    const analysis = entry({
      path: "context/analysis/a.md",
      kind: "analysis",
      status: "completed",
    });
    expect(statusDot(analysis, new Map())).toEqual({ tone: "neutral", label: "Analysis archived" });
    expect(statusDot({ ...analysis, status: "resolved" }, new Map())?.tone).toBe("neutral");
  });

  it("maps question lifecycle to unanswered and answered dots", () => {
    const question = entry({ path: "context/questions/q.md", kind: "questions", status: "active" });
    expect(statusDot(question, new Map())).toEqual({
      tone: "danger",
      label: "Question unresolved",
    });
    expect(statusDot({ ...question, status: "resolved" }, new Map())).toEqual({
      tone: "ok",
      label: "Question resolved",
    });
    expect(statusDot({ ...question, status: "draft" }, new Map())).toBeNull();
  });

  it("has no dot for other kinds", () => {
    expect(statusDot(entry({ kind: "wiki" }), new Map())).toBeNull();
    expect(statusDot(entry({ kind: "notes" }), new Map())).toBeNull();
  });

  it("lets a plan's terminal declared status decide the dot over its tasks (D4)", () => {
    // Declared completed wins even when the referenced tasks are unfinished.
    const completed = plan();
    completed.status = "completed";
    const unfinished = new Map<string, TreeEntry[]>([
      ["context/plan/x.md", [task("context/tasks/a.md", "pending")]],
    ]);
    expect(statusDot(completed, new Map(), unfinished)).toEqual({
      tone: "ok",
      label: "Plan completed",
    });

    const abandoned = plan();
    abandoned.status = "abandoned";
    expect(statusDot(abandoned, new Map(), unfinished)).toEqual({
      tone: "neutral",
      label: "Plan abandoned",
    });

    // A non-terminal declared status still derives from the task set.
    const active = plan();
    active.status = "active";
    expect(statusDot(active, new Map(), unfinished)?.tone).toBe("danger");
  });

  it("distinguishes a plan with no tasks from one with unfinished tasks", () => {
    const noneStarted = statusDot(plan(), new Map(), new Map());
    expect(noneStarted).toEqual({ tone: "danger", label: "Plan not started" });

    const unfinished = new Map<string, TreeEntry[]>([
      [
        "context/plan/x.md",
        [task("context/tasks/a.md", "in-progress"), task("context/tasks/b.md", "pending")],
      ],
    ]);
    const inFlight = statusDot(plan(), new Map(), unfinished);
    expect(inFlight).toEqual({ tone: "danger", label: "No tasks completed yet" });
    expect(inFlight?.label).not.toBe(noneStarted?.label);
  });
});

describe("groupDot", () => {
  const analysis = (name: string, status?: string) =>
    entry({ path: `context/analysis/${name}.md`, kind: "analysis", status });
  const planFor = (name: string, analysisName: string) => {
    const p = plan(`context/plan/${name}.md`);
    p.sources = [`analysis/${analysisName}.md`];
    return p;
  };

  it("aggregates none, some and all completed analyses", () => {
    const unplanned = analysis("unplanned");
    const partial = analysis("partial");
    const complete = analysis("complete");
    const entries = [
      unplanned,
      partial,
      complete,
      planFor("partial", "partial"),
      planFor("complete", "complete"),
    ];
    const plans = plansByAnalysis(entries);
    const taskIndex = new Map<string, TreeEntry[]>([
      ["context/plan/partial.md", [task("context/tasks/partial.md", "pending")]],
      ["context/plan/complete.md", [task("context/tasks/complete.md", "completed")]],
    ]);

    expect(groupDot([unplanned, partial], plans, taskIndex)).toEqual({
      tone: "danger",
      label: "0/2 analyses completed",
    });
    expect(groupDot([complete, unplanned], plans, taskIndex)).toEqual({
      tone: "warn",
      label: "1/2 analyses completed",
    });
    expect(groupDot([complete], plans, taskIndex)).toEqual({
      tone: "ok",
      label: "1/1 analyses completed",
    });
  });

  it("omits groups with no live analysis dots", () => {
    const archived = analysis("archived", "archived");
    expect(groupDot([archived], new Map())).toBeNull();
    expect(groupDot([], new Map())).toBeNull();
    expect(groupDot([entry({ kind: "questions", status: "active" })], new Map())).toBeNull();
  });
});

describe("taskProgress / tasksByPlan", () => {
  it("aggregates done/total with the red/yellow/green tones", () => {
    const tasks = (statuses: (string | undefined)[]) =>
      statuses.map((s, i) => task(`context/tasks/t${i}.md`, s));
    expect(taskProgress(tasks([""]))).toEqual({ tone: "danger", done: 0, total: 1 });
    expect(taskProgress(tasks(["completed"]))).toEqual({ tone: "ok", done: 1, total: 1 });
    expect(taskProgress(tasks(["completed", "pending"]))).toEqual({
      tone: "warn",
      done: 1,
      total: 2,
    });
    // no tasks at all → danger
    expect(taskProgress([])).toEqual({ tone: "danger", done: 0, total: 0 });
  });

  it("indexes tasks by normalized plan reference, ignoring non-tasks", () => {
    const index = tasksByPlan([
      task("context/tasks/a.md", "completed", ["plan/p.md"]),
      task("context/tasks/b.md", "pending", ["context/plan/p.md"]),
      task("context/tasks/c.md", "pending"),
      plan(),
    ]);
    expect(index.get("context/plan/p.md")?.map((e) => e.path)).toEqual([
      "context/tasks/a.md",
      "context/tasks/b.md",
    ]);
    expect(index.get("")?.map((e) => e.path)).toEqual(["context/tasks/c.md"]);
  });
});

describe("entryCompleted", () => {
  it("hides done tasks and analyses, and plans whose tasks are all done", () => {
    const index = new Map<string, TreeEntry[]>([
      ["context/plan/x.md", [task("context/tasks/a.md", "completed")]],
    ]);
    expect(entryCompleted(task("context/tasks/a.md", "completed"))).toBe(true);
    expect(entryCompleted(task("context/tasks/a.md", "pending"))).toBe(false);
    expect(entryCompleted(entry({ kind: "analysis", status: "archived" }))).toBe(true);
    expect(entryCompleted(plan())).toBe(false);
    expect(entryCompleted(plan(), index)).toBe(true);
    expect(entryCompleted(entry({ kind: "wiki", status: "completed" }))).toBe(false);
  });

  it("a plan without task references is not completed unless its status says so", () => {
    expect(entryCompleted(plan(), new Map())).toBe(false);
    const done = plan();
    done.status = "completed";
    expect(entryCompleted(done, new Map())).toBe(true);
    const abandoned = plan();
    abandoned.status = "abandoned";
    expect(entryCompleted(abandoned, new Map())).toBe(false);
  });
});

describe("isDoneStatus", () => {
  it("matches the done vocabulary case-insensitively", () => {
    for (const v of [
      "completed",
      "complete",
      "done",
      "executed",
      "archived",
      "Completed",
      " DONE ",
    ]) {
      expect(isDoneStatus(v)).toBe(true);
    }
  });

  it("is false for active/in-progress and absent statuses", () => {
    for (const v of ["active", "in-progress", "wip", "pending", "", undefined]) {
      expect(isDoneStatus(v)).toBe(false);
    }
  });
});

describe("planReferencedAnalyses", () => {
  it("collects plan sources and links as normalised corpus paths", () => {
    const entries = [
      entry({ kind: "plan", sources: ["analysis/a.md", "./analysis/b", "context/analysis/c.md"] }),
      entry({ kind: "analysis", path: "context/analysis/ignored.md", sources: ["analysis/x.md"] }),
    ];
    expect(planReferencedAnalyses(entries)).toEqual(
      new Set(["context/analysis/a.md", "context/analysis/b.md", "context/analysis/c.md"]),
    );
  });
});

describe("taskObjective", () => {
  const plans = () =>
    new Map<string, TreeEntry>([
      [
        "context/plan/x.md",
        entry({ path: "context/plan/x.md", kind: "plan", objective: "viewer" }),
      ],
    ]);

  it("inherits the objective of the plan a task sources", () => {
    expect(taskObjective(task("context/tasks/t.md", "pending", ["plan/x.md"]), plans())).toBe(
      "viewer",
    );
  });

  it("returns empty when the plan is missing or carries no objective", () => {
    expect(taskObjective(task("context/tasks/t.md", "pending", ["plan/missing.md"]), plans())).toBe(
      "",
    );
    expect(taskObjective(task("context/tasks/t.md", "pending"), new Map())).toBe("");
  });

  it("keeps a task's own objective when it carries one", () => {
    const own = task("context/tasks/t.md", "pending", ["plan/x.md"]);
    own.objective = "own-obj";
    expect(taskObjective(own, plans())).toBe("own-obj");
  });

  it("withTaskObjectives annotates only inherited tasks", () => {
    const map = plans();
    const tasks = withTaskObjectives(
      [task("context/tasks/a.md", "pending", ["plan/x.md"]), task("context/tasks/b.md", "pending")],
      map,
    );
    expect(tasks[0].objective).toBe("viewer");
    expect(tasks[1].objective).toBeUndefined();
  });
});
