// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { relationFor, siblingsOf } from "./docRelations";
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

describe("siblingsOf", () => {
  const ix = index([
    { path: "context/tasks/t1.md", kind: "tasks", title: "First" },
    { path: "context/tasks/t2.md", kind: "tasks", title: "Second" },
    { path: "context/plan/p1.md", kind: "plan", title: "Plan" },
  ]);

  it("lists the other documents in the same folder, title-sorted", () => {
    const siblings = siblingsOf("context/tasks/t1.md", ix);
    expect(siblings).toEqual([{ path: "context/tasks/t2.md", title: "Second" }]);
  });

  it("excludes the document itself and other folders", () => {
    const siblings = siblingsOf("context/tasks/t1.md", ix);
    expect(siblings.some((s) => s.path === "context/plan/p1.md")).toBe(false);
  });

  it("is empty without an index", () => {
    expect(siblingsOf("context/tasks/t1.md", null)).toEqual([]);
  });
});
