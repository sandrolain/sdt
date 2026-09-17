/// <reference types="node" />
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

// Read the source stylesheet: vitest stubs every CSS import to an empty string,
// which would silently defeat these assertions.
const css = readFileSync(new URL("./index.css", import.meta.url), "utf8");

/** Declaration block of the first `selector { … }` rule in the stylesheet. */
function block(selector: string): string {
  const start = css.indexOf(`${selector} {`);
  if (start < 0) throw new Error(`missing rule: ${selector}`);
  const open = css.indexOf("{", start);
  const close = css.indexOf("}", open);
  return css.slice(open + 1, close);
}

/**
 * Regression guard for the Code-mode vertical scroll (round-7 item 2): the
 * document tab must be a flex column whose code surface is the single bounded
 * scroller. An `overscroll-behavior: contain` here previously trapped the wheel
 * over the code block.
 */
describe("document scroll structure", () => {
  it("makes the document tab a bounded flex column", () => {
    const tab = block(".dock-content.doc-tab");
    expect(tab).toContain("display: flex");
    expect(tab).toContain("flex-direction: column");
    expect(tab).toContain("overflow: hidden");
    expect(tab).toContain("min-height: 0");
  });

  it("bounds the code surface and lets the wheel chain out", () => {
    const wrap = block(".doc-code-wrap");
    expect(wrap).toContain("flex: 1 1 auto");
    expect(wrap).toContain("min-height: 0");
    expect(wrap).toContain("overflow: auto");
    expect(wrap).not.toContain("overscroll-behavior");
  });

  it("keeps render and canvas modes scrollable too", () => {
    expect(block(".doc-rendered")).toContain("overflow: auto");
    expect(block(".dock-content.doc-tab .doc-view")).toContain("overflow: auto");
  });
});
