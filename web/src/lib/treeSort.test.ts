// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { TreeEntry } from "./api";
import { KIND_ORDER } from "./kinds";
import {
  folderCount,
  groupByFolder,
  groupByKind,
  groupByObjective,
  groupByPlan,
  groupDate,
  groupSort,
  latestDate,
  sortEntries,
} from "./treeSort";

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

describe("groupByObjective", () => {
  it("buckets by slug with named groups sorted, ungrouped last", () => {
    const groups = groupByObjective([
      entry({ path: "context/analysis/a.md", objective: "viewer" }),
      entry({ path: "context/analysis/b.md", objective: "memory" }),
      entry({ path: "context/analysis/c.md", objective: "viewer" }),
      entry({ path: "context/analysis/d.md" }),
    ]);
    expect(groups.map((g) => g.objective)).toEqual(["memory", "viewer", ""]);
    expect(groups.find((g) => g.objective === "viewer")?.entries.map((e) => e.path)).toEqual([
      "context/analysis/a.md",
      "context/analysis/c.md",
    ]);
    expect(groups.find((g) => g.objective === "")?.entries.map((e) => e.path)).toEqual([
      "context/analysis/d.md",
    ]);
  });

  it("omits the ungrouped bucket when every entry has an objective", () => {
    const groups = groupByObjective([
      entry({ path: "context/analysis/a.md", objective: "viewer" }),
    ]);
    expect(groups.map((g) => g.objective)).toEqual(["viewer"]);
  });

  it("returns only the ungrouped bucket when no objective is set", () => {
    const groups = groupByObjective([entry({ path: "context/analysis/a.md" })]);
    expect(groups.map((g) => g.objective)).toEqual([""]);
  });
});

describe("groupByFolder", () => {
  it("nests deeper wiki entries under path-derived folders, root entries flat", () => {
    const entries = [
      entry({ path: "context/wiki/root.md", kind: "wiki" }),
      entry({ path: "context/wiki/regulations/a.md", kind: "wiki" }),
      entry({ path: "context/wiki/regulations/b.md", kind: "wiki" }),
      entry({ path: "context/wiki/regulations/sub/c.md", kind: "wiki" }),
    ];
    const { rootEntries, folders } = groupByFolder(entries, "context/wiki/");
    expect(rootEntries.map((e) => e.path)).toEqual(["context/wiki/root.md"]);
    expect(folders.map((f) => f.name)).toEqual(["regulations"]);
    const reg = folders[0];
    expect(reg.entries.map((e) => e.path)).toEqual([
      "context/wiki/regulations/a.md",
      "context/wiki/regulations/b.md",
    ]);
    expect(reg.children.map((c) => c.name)).toEqual(["sub"]);
    expect(reg.children[0].entries.map((e) => e.path)).toEqual([
      "context/wiki/regulations/sub/c.md",
    ]);
    expect(folderCount(reg)).toBe(3);
  });

  it("keeps entries outside the prefix at the root", () => {
    const { rootEntries } = groupByFolder(
      [entry({ path: "context/notes/x.md", kind: "notes" })],
      "context/wiki/",
    );
    expect(rootEntries.map((e) => e.path)).toEqual(["context/notes/x.md"]);
  });
});

describe("groupByPlan", () => {
  const plans = new Map<string, TreeEntry>([
    [
      "context/plan/20260920-a-plan.md",
      entry({
        path: "context/plan/20260920-a-plan.md",
        kind: "plan",
        title: "Plan A",
        created: "2026-09-20",
      }),
    ],
    [
      "context/plan/20260919-b-plan.md",
      entry({
        path: "context/plan/20260919-b-plan.md",
        kind: "plan",
        title: "Plan B",
        created: "2026-09-19",
      }),
    ],
  ]);

  it("groups tasks under their plan, ordered by plan created desc, ungrouped last", () => {
    const groups = groupByPlan(
      [
        entry({ path: "context/tasks/t1.md", kind: "tasks", sources: ["plan/20260920-a-plan.md"] }),
        entry({
          path: "context/tasks/t2.md",
          kind: "tasks",
          sources: ["context/plan/20260919-b-plan.md"],
        }),
        entry({ path: "context/tasks/t3.md", kind: "tasks", sources: ["plan/20260920-a-plan.md"] }),
        entry({ path: "context/tasks/t4.md", kind: "tasks" }),
      ],
      plans,
    );
    expect(groups.map((g) => g.label)).toEqual(["Plan A", "Plan B", ""]);
    expect(groups[0].entries.map((e) => e.path)).toEqual([
      "context/tasks/t1.md",
      "context/tasks/t3.md",
    ]);
    expect(groups[2].entries.map((e) => e.path)).toEqual(["context/tasks/t4.md"]);
  });

  it("labels a plan group from the filename slug when the plan entry is absent", () => {
    const groups = groupByPlan(
      [entry({ path: "context/tasks/t1.md", kind: "tasks", sources: ["plan/20260920-a-plan.md"] })],
      new Map(),
    );
    expect(groups.map((g) => g.label)).toEqual(["a-plan"]);
  });

  it("ignores non-plan references and leaves such tasks ungrouped", () => {
    const groups = groupByPlan(
      [
        entry({
          path: "context/tasks/t1.md",
          kind: "tasks",
          sources: ["analysis/20260920-a.md"],
        }),
      ],
      plans,
    );
    expect(groups.map((g) => g.label)).toEqual([""]);
  });
});

describe("groupSort", () => {
  it("returns a date descriptor only for date sort keys", () => {
    expect(groupSort("created_asc")).toEqual({ field: "created", dir: "asc" });
    expect(groupSort("created_desc")).toEqual({ field: "created", dir: "desc" });
    expect(groupSort("modified_asc")).toEqual({ field: "modified", dir: "asc" });
    expect(groupSort("modified_desc")).toEqual({ field: "modified", dir: "desc" });
    expect(groupSort("name_asc")).toBeNull();
    expect(groupSort("title_desc")).toBeNull();
  });
});

describe("latestDate / groupDate", () => {
  it("returns the max raw date for a field, empty when none", () => {
    const entries = [
      entry({ path: "context/notes/a.md", created: "2026-09-02", modified: "2026-09-03" }),
      entry({
        path: "context/notes/b.md",
        created: "2026-09-05T10:00:00Z",
        modified: "2026-09-04",
      }),
      entry({ path: "context/notes/c.md" }),
    ];
    expect(latestDate(entries, "created")).toBe("2026-09-05T10:00:00Z");
    expect(latestDate(entries, "modified")).toBe("2026-09-04");
    expect(latestDate([entry({}), entry({})], "created")).toBe("");
  });

  it("groupDate mirrors the per-entry line: created, else filename date prefix", () => {
    const entries = [
      entry({ path: "context/wiki/20260910-000000-no-created.md" }),
      entry({ path: "context/wiki/with-created.md", created: "2026-09-12" }),
    ];
    expect(groupDate(entries)).toBe("2026-09-12");
    expect(groupDate([entry({ path: "context/wiki/20260910-000000-no-created.md" })])).toBe(
      "2026-09-10",
    );
    expect(groupDate([entry({ path: "context/wiki/no-date.md" })])).toBe("");
  });
});

describe("date-ranked group ordering", () => {
  it("groupByObjective orders named groups by the latest created, ungrouped last", () => {
    const groups = groupByObjective(
      [
        entry({ path: "context/analysis/a.md", objective: "old", created: "2026-09-01" }),
        entry({ path: "context/analysis/b.md", objective: "new", created: "2026-09-20" }),
        entry({ path: "context/analysis/c.md", objective: "old", created: "2026-09-02" }),
        entry({ path: "context/analysis/d.md" }),
      ],
      { field: "created", dir: "desc" },
    );
    expect(groups.map((g) => g.objective)).toEqual(["new", "old", ""]);
  });

  it("groupByObjective keeps name order without a sort descriptor", () => {
    const groups = groupByObjective([
      entry({ path: "context/analysis/a.md", objective: "viewer" }),
      entry({ path: "context/analysis/b.md", objective: "memory" }),
    ]);
    expect(groups.map((g) => g.objective)).toEqual(["memory", "viewer"]);
  });

  it("groupByPlan orders named groups by the latest task date under a date sort", () => {
    const plans = new Map<string, TreeEntry>([
      [
        "context/plan/20260920-a-plan.md",
        entry({
          path: "context/plan/20260920-a-plan.md",
          kind: "plan",
          title: "Plan A",
          created: "2026-09-20",
        }),
      ],
      [
        "context/plan/20260919-b-plan.md",
        entry({
          path: "context/plan/20260919-b-plan.md",
          kind: "plan",
          title: "Plan B",
          created: "2026-09-19",
        }),
      ],
    ]);
    const groups = groupByPlan(
      [
        entry({
          path: "context/tasks/t1.md",
          kind: "tasks",
          sources: ["plan/20260920-a-plan.md"],
          created: "2026-09-30",
        }),
        entry({
          path: "context/tasks/t2.md",
          kind: "tasks",
          sources: ["context/plan/20260919-b-plan.md"],
          created: "2026-09-10",
        }),
      ],
      plans,
      { field: "created", dir: "asc" },
    );
    // oldest latest-task-date first, overturning the plan-created-desc default
    expect(groups.map((g) => g.label)).toEqual(["Plan B", "Plan A"]);
  });

  it("groupByFolder orders folders by the latest date across their subtree", () => {
    const entries = [
      entry({ path: "context/wiki/zzz/a.md", kind: "wiki", created: "2026-09-05" }),
      entry({ path: "context/wiki/aaa/b.md", kind: "wiki", created: "2026-09-20" }),
      entry({ path: "context/wiki/aaa/sub/c.md", kind: "wiki", created: "2026-09-25" }),
    ];
    const { folders } = groupByFolder(entries, "context/wiki/", { field: "created", dir: "desc" });
    expect(folders.map((f) => f.name)).toEqual(["aaa", "zzz"]);
  });

  it("groupByFolder keeps name order without a sort descriptor", () => {
    const entries = [
      entry({ path: "context/wiki/zzz/a.md", kind: "wiki", created: "2026-09-05" }),
      entry({ path: "context/wiki/aaa/b.md", kind: "wiki", created: "2026-09-20" }),
    ];
    const { folders } = groupByFolder(entries, "context/wiki/");
    expect(folders.map((f) => f.name)).toEqual(["aaa", "zzz"]);
  });

  it("date-ranked ordering keeps groups with no date last", () => {
    const entries = [
      entry({ path: "context/wiki/aaa/b.md", kind: "wiki", created: "2026-09-20" }),
      entry({ path: "context/wiki/zzz/a.md", kind: "wiki" }),
    ];
    const { folders } = groupByFolder(entries, "context/wiki/", { field: "created", dir: "desc" });
    expect(folders.map((f) => f.name)).toEqual(["aaa", "zzz"]);
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
