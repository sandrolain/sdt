import { describe, expect, it } from "vitest";
import type { TreeEntry } from "./api";
import { availableKinds, entryKind, KIND_ORDER, kindFromPath, kindLabel } from "./kinds";

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

  it("uses proposal and research kinds", () => {
    expect(entryKind(entry({ kind: "proposal" }))).toBe("proposal");
    expect(entryKind(entry({ kind: "research" }))).toBe("research");
  });

  it("uses the decision kind for decision records", () => {
    expect(entryKind(entry({ kind: "decision" }))).toBe("decision");
  });

  it("uses the commands kind for command-trigger files", () => {
    expect(entryKind(entry({ kind: "commands" }))).toBe("commands");
  });

  it("treats mermaid entries as their own kind", () => {
    expect(entryKind(entry({ mermaid: true, kind: "mermaid" }))).toBe("mermaid");
    expect(entryKind(entry({ mermaid: true }))).toBe("mermaid");
  });
});

describe("kindFromPath", () => {
  it("maps corpus folders to kinds, including commands and plurals", () => {
    expect(kindFromPath("context/commands/index.md")).toBe("commands");
    expect(kindFromPath("context/proposals/p.md")).toBe("proposal");
    expect(kindFromPath("context/questions/q.md")).toBe("questions");
    expect(kindFromPath("context/research/r.md")).toBe("research");
    expect(kindFromPath("context/wiki/a.md")).toBe("wiki");
    expect(kindFromPath("context/unknown/x.md")).toBe("other");
  });

  it("maps .mmd paths to the mermaid kind", () => {
    expect(kindFromPath("context/wiki/flow.mmd")).toBe("mermaid");
  });
});

describe("kindLabel", () => {
  it("labels kinds with human plurals", () => {
    expect(kindLabel("analysis")).toBe("Analyses");
    expect(kindLabel("notes")).toBe("Notes");
    expect(kindLabel("plan")).toBe("Plans");
    expect(kindLabel("worklog")).toBe("Work log");
    expect(kindLabel("decision")).toBe("Decisions");
    expect(kindLabel("mermaid")).toBe("Diagrams");
    expect(kindLabel("other")).toBe("Other");
  });

  it("has a label for every kind", () => {
    for (const kind of KIND_ORDER) {
      expect(kindLabel(kind)).toBeTruthy();
    }
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

  it("orders proposal before research", () => {
    const kinds = availableKinds([entry({ kind: "research" }), entry({ kind: "proposal" })]);
    expect(kinds.indexOf("proposal")).toBeLessThan(kinds.indexOf("research"));
  });
});
