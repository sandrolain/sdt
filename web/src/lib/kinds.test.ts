import { describe, expect, it } from "vitest";
import type { TreeEntry } from "./api";
import { availableKinds, entryKind, kindLabel } from "./kinds";

function entry(patch: Partial<TreeEntry>): TreeEntry {
  return { path: "context/x/a.md", ...patch };
}

describe("entryKind", () => {
  it("uses known frontmatter kinds", () => {
    expect(entryKind(entry({ kind: "analysis" }))).toBe("analysis");
    expect(entryKind(entry({ kind: "wiki" }))).toBe("wiki");
  });

  it("maps unknown/empty kinds to other", () => {
    expect(entryKind(entry({ kind: "nope" }))).toBe("other");
    expect(entryKind(entry({}))).toBe("other");
  });

  it("treats canvas entries as canvas regardless of kind", () => {
    expect(entryKind(entry({ canvas: true, kind: "wiki" }))).toBe("canvas");
  });
});

describe("kindLabel", () => {
  it("labels kinds verbatim, canvas singular", () => {
    expect(kindLabel("analysis")).toBe("analysis");
    expect(kindLabel("notes")).toBe("notes");
    expect(kindLabel("canvas")).toBe("canvas");
  });
});

describe("availableKinds", () => {
  it("lists entries in corpus order with canvas appended", () => {
    const kinds = availableKinds([
      entry({ kind: "worklog" }),
      entry({ kind: "analysis" }),
      entry({ canvas: true }),
      entry({ kind: "wiki" }),
      entry({ kind: "nope" }),
    ]);
    expect(kinds.indexOf("analysis")).toBeLessThan(kinds.indexOf("worklog"));
    expect(kinds[kinds.length - 1]).toBe("canvas");
    expect(kinds).toContain("other");
  });
});
