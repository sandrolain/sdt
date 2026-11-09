// @vitest-environment node
import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { detectType } from "pele";
import { describe, expect, it } from "vitest";

/**
 * The diagram keyword set has two copies, and this is one of the two tests that
 * bind them. The other is `TestDiagramKeywordsMatchDialect` in
 * `cli/cmd/context_lint_diagram_test.go`.
 *
 * This half asks the renderer itself: every keyword the Go lint accepts must be
 * a type the pinned Pele detects, so the check cannot claim to know a form the
 * viewer cannot draw — and it cannot fall behind a renderer upgrade either,
 * because the answer comes from the installed parser registry rather than from
 * a list kept here. The second case holds the dialect module's own table to the
 * same set, which is what keeps the table and the check from drifting apart.
 *
 * The corpus half skips when `context/` is absent: a fresh clone has no corpus,
 * and a silent empty pass would hide that.
 */
const GO_LINT = fileURLToPath(new URL("../../../cli/cmd/context_lint_diagram.go", import.meta.url));
const DIALECT = fileURLToPath(
  new URL("../../../context/instructions/diagram-mermaid.md", import.meta.url),
);

/** The keywords of the Go lint's closed set, read from its map literal. */
function lintKeywords(): string[] {
  const source = readFileSync(GO_LINT, "utf8");
  const block = /var ctxDiagramKeywords = map\[string\]bool\{([\s\S]*?)\n\}/.exec(source);
  expect(block, "ctxDiagramKeywords not found in the Go lint").not.toBeNull();
  return [...block![1].matchAll(/"([^"]+)":\s*true/g)].map((match) => match[1]);
}

/** The backticked keywords of the dialect module's question → keyword table. */
function dialectKeywords(): string[] {
  const keywords: string[] = [];
  let inTable = false;
  for (const line of readFileSync(DIALECT, "utf8").split("\n")) {
    const trimmed = line.trim();
    if (trimmed.startsWith("| Question")) {
      inTable = true;
      continue;
    }
    if (!inTable) continue;
    if (!trimmed.startsWith("|")) break;
    const cells = trimmed.split("|");
    if (cells.length < 3) continue;
    for (const match of cells[2].matchAll(/`([^`]+)`/g)) keywords.push(match[1]);
  }
  return keywords;
}

describe("the diagram keyword set", () => {
  it("lists only keywords the pinned renderer detects", () => {
    const keywords = lintKeywords();
    expect(keywords.length).toBeGreaterThan(0);
    for (const keyword of keywords) {
      expect(
        detectType(`${keyword}\n`),
        `${keyword}: accepted by the lint, not detected by Pele`,
      ).toBeTruthy();
    }
  });

  it.skipIf(!existsSync(DIALECT))("carries every keyword the dialect module names", () => {
    const set = new Set(lintKeywords());
    const named = dialectKeywords();
    expect(named.length).toBeGreaterThan(0);
    for (const keyword of named) {
      expect(
        set.has(keyword),
        `${keyword}: named by the dialect, missing from the lint's set`,
      ).toBe(true);
    }
  });
});
