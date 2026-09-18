// @vitest-environment node
import { describe, expect, it } from "vitest";
import { isExcludedPath, normalizeCorpusPath } from "./exclusions";

describe("normalizeCorpusPath", () => {
  it("collapses dot segments and separators", () => {
    expect(normalizeCorpusPath("context/./wiki/../plan/x.md")).toBe("context/plan/x.md");
    expect(normalizeCorpusPath("context\\refs\\a.md")).toBe("context/refs/a.md");
  });
});

describe("isExcludedPath", () => {
  it("matches excluded dir segments", () => {
    for (const p of [
      "context/tmp/x.md",
      "context/scripts/a.go",
      "context/refs/repo/a.md",
      "context/instructions/plan.md",
      "context/sdtdocs/README.md",
    ]) {
      expect(isExcludedPath(p)).toBe(true);
    }
  });

  it("matches the corpus README only", () => {
    expect(isExcludedPath("context/README.md")).toBe(true);
    expect(isExcludedPath("context/notes/README.md")).toBe(false);
  });

  it("keeps ordinary paths", () => {
    for (const p of [
      "context/wiki/topic.md",
      "context/wiki/commands.md",
      "context/commands/index.md",
      "context/plan/x.md",
    ]) {
      expect(isExcludedPath(p)).toBe(false);
    }
  });
});
