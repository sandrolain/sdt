// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { relationFor } from "./docRelations";
import type { CorpusIndex } from "./corpusIndex";
import type { TreeEntry } from "./api";

function index(entries: TreeEntry[]): CorpusIndex {
  const map: CorpusIndex = new Map();
  for (const entry of entries) map.set(entry.path, { entry, kind: "other" });
  return map;
}

describe("relationFor", () => {
  it("names the plan a task file belongs to", () => {
    const ix = index([
      { path: "context/tasks/t1.md", kind: "tasks", plan: "context/plan/p1.md" },
      { path: "context/plan/p1.md", kind: "plan", title: "Plan one" },
    ]);
    expect(relationFor("context/tasks/t1.md", ix)).toEqual({
      label: "Plan",
      path: "context/plan/p1.md",
      title: "Plan one",
    });
  });

  it("names the analysis a plan derives from", () => {
    const ix = index([
      { path: "context/plan/p1.md", kind: "plan", analysis: "context/analysis/a1.md" },
      { path: "context/analysis/a1.md", kind: "analysis" },
    ]);
    expect(relationFor("context/plan/p1.md", ix)?.label).toBe("Analysis");
  });

  it("falls back to the parent path when the parent has no title", () => {
    const ix = index([
      { path: "context/tasks/t1.md", kind: "tasks", plan: "context/plan/p1.md" },
      { path: "context/plan/p1.md", kind: "plan" },
    ]);
    expect(relationFor("context/tasks/t1.md", ix)?.title).toBe("P1");
  });

  it("has nothing to link without an index or a typed edge", () => {
    expect(relationFor("context/notes/n.md", null)).toBeNull();
    expect(relationFor("context/notes/n.md", index([{ path: "context/notes/n.md" }]))).toBeNull();
  });
});
