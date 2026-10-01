/// <reference types="node" />
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

// Print contract (plan phase 11): printing a document yields the document and
// nothing else. This guard protects the four parts that are easy to break
// silently: the chrome hidden, the pane layout released, the code/table
// overflow expanded, and the print palette pinned in the token layer.

const css = readFileSync(new URL("./index.css", import.meta.url), "utf8");
const tokens = readFileSync(new URL("./tokens.css", import.meta.url), "utf8");

/** The trailing `@media print` block of a stylesheet. */
function printBlock(source: string): string {
  const start = source.indexOf("@media print");
  if (start < 0) throw new Error("missing @media print block");
  return source.slice(start);
}

describe("print stylesheet", () => {
  const block = printBlock(css);

  it("drops the shell chrome", () => {
    for (const selector of [
      ".top-bar",
      ".panel--tree",
      ".panel--meta",
      ".doc-path",
      ".doc-progress",
      ".dv-tabs-and-actions-container",
    ]) {
      expect(block).toContain(selector);
    }
    expect(block).toContain("display: none !important");
  });

  it("releases the pane layout so the document reaches the paper", () => {
    // dockview writes inline height/width/position on its wrappers, so the
    // release has to out-specify inline styles — otherwise the pane collapses
    // to 0px and the printed page comes out blank.
    for (const selector of [".app-shell", ".dv-groupview", ".dv-view", ".doc-rendered"]) {
      expect(block).toContain(selector);
    }
    expect(block).toContain("height: auto !important");
    expect(block).toContain("overflow: visible !important");
    expect(block).toContain("position: static !important");
    expect(block).toContain("--reading-measure: none");
  });

  it("expands code and table overflow instead of clipping it", () => {
    expect(block).toContain("white-space: pre-wrap");
    expect(block).toContain("overflow-wrap: anywhere");
    expect(block).toContain(".md-table-wrap table");
    expect(block).toContain(".doc-rendered > *");
  });

  it("keeps links legible on paper", () => {
    expect(block).toContain(".doc-rendered a {");
    expect(block).toContain("color: var(--accent)");
    expect(block).toContain("text-decoration: underline");
  });

  it("pins a black-on-white palette in the token layer, in both themes", () => {
    const printTokens = printBlock(tokens);
    expect(printTokens).toContain('[data-theme="dark"]');
    expect(printTokens).toContain('[data-theme="light"]');
    expect(printTokens).toMatch(/--fg:\s*#000000/);
    expect(printTokens).toMatch(/--bg-base:\s*#ffffff/);
    // the raw values stay out of the component stylesheet
    expect(css).not.toMatch(/--fg:\s*#/);
    expect(css).not.toMatch(/--bg-base:\s*#/);
  });
});
