// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { TreeEntry } from "./api";
import { planReferencedAnalyses, statusDot } from "./statusDot";

function entry(patch: Partial<TreeEntry>): TreeEntry {
  return { path: "context/plan/x.md", kind: "plan", ...patch };
}

describe("statusDot", () => {
  it("marks plans by execution state", () => {
    expect(statusDot(entry({ status: "completed" }), new Set())?.tone).toBe("ok");
    expect(statusDot(entry({ status: "active" }), new Set())?.tone).toBe("danger");
  });

  it("shows no dot when the status is unknown", () => {
    expect(statusDot(entry({ status: undefined }), new Set())).toBeNull();
    expect(statusDot(entry({ kind: "tasks", status: "" }), new Set())).toBeNull();
  });

  it("marks tasks by execution state", () => {
    const task = (status?: string) => entry({ path: "context/tasks/t.md", kind: "tasks", status });
    expect(statusDot(task("completed"), new Set())?.tone).toBe("ok");
    expect(statusDot(task("in-progress"), new Set())?.tone).toBe("warn");
    expect(statusDot(task("active"), new Set())?.tone).toBe("danger");
  });

  it("flags an analysis without a plan and clears a planned one", () => {
    const analysis = entry({ path: "context/analysis/a.md", kind: "analysis" });
    expect(statusDot(analysis, new Set())).toEqual({
      tone: "warn",
      label: "Analysis without a plan",
    });
    expect(statusDot(analysis, new Set(["context/analysis/a.md"]))).toBeNull();
  });

  it("has no dot for other kinds", () => {
    expect(statusDot(entry({ kind: "wiki" }), new Set())).toBeNull();
    expect(statusDot(entry({ kind: "notes" }), new Set())).toBeNull();
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
