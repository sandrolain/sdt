// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { TreeEntry } from "./api";
import {
  entryCompleted,
  isDoneStatus,
  planReferencedAnalyses,
  statusDot,
  taskProgress,
  tasksByPlan,
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
    expect(statusDot(plan(), new Set(), empty)).toEqual({
      tone: "danger",
      label: "Plan not started",
    });

    const noneDone = new Map<string, TreeEntry[]>([
      [
        "context/plan/x.md",
        [task("context/tasks/a.md", "pending"), task("context/tasks/b.md", "wip")],
      ],
    ]);
    expect(statusDot(plan(), new Set(), noneDone)?.tone).toBe("danger");

    const partial = new Map<string, TreeEntry[]>([
      [
        "context/plan/x.md",
        [task("context/tasks/a.md", "completed"), task("context/tasks/b.md", "pending")],
      ],
    ]);
    const partialDot = statusDot(plan(), new Set(), partial);
    expect(partialDot).toEqual({ tone: "warn", label: "Plan in progress" });

    const allDone = new Map<string, TreeEntry[]>([
      [
        "context/plan/x.md",
        [task("context/tasks/a.md", "completed"), task("context/tasks/b.md", "archived")],
      ],
    ]);
    expect(statusDot(plan(), new Set(), allDone)).toEqual({
      tone: "ok",
      label: "Plan completed",
    });
  });

  it("marks tasks by execution state, empty status red", () => {
    const t = (status?: string) => task("context/tasks/t.md", status);
    expect(statusDot(t("completed"), new Set())).toEqual({ tone: "ok", label: "Task completed" });
    expect(statusDot(t("in-progress"), new Set())).toEqual({
      tone: "warn",
      label: "Task in progress",
    });
    expect(statusDot(t(""), new Set())).toEqual({ tone: "danger", label: "Task not started" });
    expect(statusDot(t(undefined), new Set())?.tone).toBe("danger");
    expect(statusDot(t("active"), new Set())?.tone).toBe("danger");
  });

  it("flags an analysis without a plan and clears a planned one", () => {
    const analysis = entry({ path: "context/analysis/a.md", kind: "analysis" });
    expect(statusDot(analysis, new Set())).toEqual({
      tone: "warn",
      label: "Analysis without a plan",
    });
    expect(statusDot(analysis, new Set(["context/analysis/a.md"]))).toBeNull();
  });

  it("keeps the analysis dot null once the status is done", () => {
    const analysis = entry({
      path: "context/analysis/a.md",
      kind: "analysis",
      status: "completed",
    });
    expect(statusDot(analysis, new Set())).toBeNull();
  });

  it("has no dot for other kinds", () => {
    expect(statusDot(entry({ kind: "wiki" }), new Set())).toBeNull();
    expect(statusDot(entry({ kind: "notes" }), new Set())).toBeNull();
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

  it("a plan without task references is never completed (not hidden)", () => {
    expect(entryCompleted(plan(), new Map())).toBe(false);
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
