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

  it("truncates tree entry titles and drops the per-entry kind badge", () => {
    const title = block(".tree-entry__title");
    expect(title).toContain("text-overflow: ellipsis");
    expect(title).toContain("white-space: nowrap");
    // kind identity lives in the folder header; the badge rules are removed
    expect(css.indexOf(".tree-entry__kind {")).toBe(-1);
    expect(css.indexOf(".tree-entry__kind--canvas")).toBe(-1);
  });

  it("lets the tree panel shrink below its content min-width", () => {
    expect(block(".panel--tree")).toContain("min-width: 0");
    expect(block(".panel--tree")).toContain("overflow-x: hidden");
  });

  it("lets the meta panel and the path row shrink", () => {
    expect(block(".panel--meta")).toContain("min-width: 0");
    expect(block(".panel--meta")).toContain("overflow-x: hidden");
    expect(block(".doc-path")).toContain("min-width: 0");
    const value = block(".doc-path__value");
    expect(value).toContain("min-width: 0");
    // the path wraps instead of ellipsizing; no single-line nowrap
    expect(value).toContain("overflow-wrap: anywhere");
    expect(value).toContain("white-space: normal");
    expect(value).not.toContain("white-space: nowrap");
    expect(value).not.toContain("text-overflow: ellipsis");
    // the RTL trick reported a large min-content width and blocked shrinking
    expect(value).not.toContain("direction: rtl");
    expect(block(".meta-row")).toContain("minmax(0, 1fr)");
    expect(block(".doc-rendered")).toContain("min-width: 0");
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

  it("sticks the tree toolbar above the folders", () => {
    const toolbar = block(".tree-toolbar");
    expect(toolbar).toContain("position: sticky");
    expect(toolbar).toContain("top: 0");
    expect(toolbar).toContain("flex-wrap: wrap");
  });
});
