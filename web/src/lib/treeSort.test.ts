// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { TreeEntry } from "./api";
import { KIND_ORDER } from "./kinds";
import { groupByKind, sortEntries } from "./treeSort";

function entry(patch: Partial<TreeEntry>): TreeEntry {
  return { path: "context/notes/x.md", ...patch };
}

const ENTRIES: TreeEntry[] = [
  entry({ path: "context/wiki/beta.md", kind: "wiki", title: "Beta", created: "2026-09-02" }),
  entry({
    path: "context/wiki/20260915-195559-alpha-plan.md",
    kind: "wiki",
    title: "Alpha",
    created: "2026-09-01",
    modified: "2026-09-10T10:00:00Z",
  }),
  entry({
    path: "context/notes/gamma.md",
    kind: "notes",
    title: "Gamma",
    modified: "2026-09-05T08:00:00Z",
  }),
];

describe("groupByKind", () => {
  it("returns every known kind folder in KIND_ORDER, canvas last", () => {
    const groups = groupByKind([
      entry({ path: "context/board.canvas", canvas: true }),
      entry({ path: "context/notes/a.md", kind: "notes" }),
      entry({ path: "context/wiki/b.md", kind: "wiki" }),
    ]);
    const expected = [...KIND_ORDER, "canvas"];
    expect(groups.map((g) => g.kind)).toEqual(expected);
    // empty folders are present with zero entries
    expect(groups.find((g) => g.kind === "questions")?.entries).toHaveLength(0);
    expect(groups.find((g) => g.kind === "commands")?.entries).toHaveLength(0);
    expect(groups.find((g) => g.kind === "wiki")?.entries).toHaveLength(1);
    expect(groups.find((g) => g.kind === "canvas")?.entries).toHaveLength(1);
  });

  it("omits the canvas group when no canvas entries exist", () => {
    const groups = groupByKind([entry({ path: "context/notes/a.md", kind: "notes" })]);
    expect(groups.map((g) => g.kind)).toEqual(KIND_ORDER);
  });
});

describe("sortEntries", () => {
  it("sorts by name ascending by default", () => {
    const names = sortEntries(ENTRIES, "name_asc").map((e) => e.path);
    expect(names).toEqual([
      "context/wiki/20260915-195559-alpha-plan.md",
      "context/wiki/beta.md",
      "context/notes/gamma.md",
    ]);
  });

  it("sorts by title descending", () => {
    const titles = sortEntries(ENTRIES, "title_desc").map((e) => e.title ?? e.path);
    expect(titles).toEqual(["Gamma", "Beta", "Alpha"]);
  });

  it("sorts by created ascending with missing values last", () => {
    const created = sortEntries(ENTRIES, "created_asc").map((e) => e.created ?? "");
    expect(created).toEqual(["2026-09-01", "2026-09-02", ""]);
  });

  it("sorts by modified descending", () => {
    const modified = sortEntries(ENTRIES, "modified_desc").map((e) => e.modified ?? "");
    expect(modified).toEqual(["2026-09-10T10:00:00Z", "2026-09-05T08:00:00Z", ""]);
  });

  it("does not mutate the input", () => {
    const before = ENTRIES.map((e) => e.path);
    sortEntries(ENTRIES, "title_asc");
    expect(ENTRIES.map((e) => e.path)).toEqual(before);
  });
});
