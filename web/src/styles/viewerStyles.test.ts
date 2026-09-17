/// <reference types="node" />
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

// Read the source stylesheets: vitest stubs CSS imports to an empty string.
const css = readFileSync(new URL("./index.css", import.meta.url), "utf8");
const dockviewCss = readFileSync(new URL("./dockview.css", import.meta.url), "utf8");

/** Declaration block of the first `selector { … }` rule in the stylesheet. */
function block(selector: string): string {
  const start = css.indexOf(`${selector} {`);
  if (start < 0) throw new Error(`missing rule: ${selector}`);
  const open = css.indexOf("{", start);
  const close = css.indexOf("}", open);
  return css.slice(open + 1, close);
}

describe("viewer styles", () => {
  it("makes the active tree entry stand out over hover", () => {
    const active = block(".tree-entry.is-active");
    expect(active).toContain("color-mix");
    expect(active).toContain("font-weight: 600");
    expect(block(".tree-entry:hover")).not.toContain("color-mix");
  });

  it("styles the rendered hr and richer typography", () => {
    expect(block(".doc-rendered hr")).toContain("border-top");
    expect(block(".doc-rendered a")).toContain("--ctp-blue");
    expect(block(".doc-rendered blockquote")).toContain("border-left: 3px solid var(--accent)");
  });

  it("does not paint a duplicate background on .dv-react-part", () => {
    const start = dockviewCss.indexOf(".dv-groupview,");
    const open = dockviewCss.indexOf("{", start);
    const selector = dockviewCss.slice(start, open);
    expect(selector).toContain(".dock-content");
    expect(selector).not.toContain(".dv-react-part");
  });

  it("gives the tab actions the full strip height", () => {
    const actions = block(".doc-tab-actions");
    expect(actions).toContain("height: 100%");
    expect(actions).toContain("align-items: center");
  });
});
