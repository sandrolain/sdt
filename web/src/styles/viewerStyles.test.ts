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
    const tree = block(".panel--tree");
    expect(tree).toContain("min-width: 0");
    // a fixed toolbar plus a scroll region, so the headers can stick to it
    expect(tree).toContain("display: flex");
    expect(tree).toContain("overflow: hidden");
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

  it("paints only the custom .dock-content, never the dockview surfaces", () => {
    const start = dockviewCss.indexOf(".dock-content");
    const open = dockviewCss.indexOf("{", start);
    const selector = dockviewCss.slice(start, open);
    expect(selector).toContain(".dock-content");
    expect(selector).not.toContain(".dv-react-part");
    expect(selector).not.toContain(".dv-groupview");
  });

  it("gives the tab actions the full strip height", () => {
    const actions = block(".doc-tab-actions");
    expect(actions).toContain("height: 100%");
    expect(actions).toContain("align-items: center");
  });

  it("resets list bullets across the whole tree, flat mode included", () => {
    const reset = block(".tree-groups ul");
    expect(reset).toContain("list-style: none");
    expect(reset).toContain("margin: 0");
    expect(reset).toContain("padding: 0");
    // the reset is no longer scoped to kind folders, and flat mode needs no rule
    expect(css.indexOf(".tree-folder ul {")).toBe(-1);
    expect(css.indexOf(".tree-flat {")).toBe(-1);
  });

  it("keeps the tree toolbar fixed and sticks only the kind headers", () => {
    const toolbar = block(".tree-toolbar");
    expect(toolbar).toContain("flex-wrap: wrap");
    expect(toolbar).not.toContain("position: sticky");
    expect(block(".tree-groups")).toContain("overflow: auto");
    const headers = block(".tree-groups > .tree-folder > .tree-folder__header");
    expect(headers).toContain("position: sticky");
    expect(headers).toContain("top: 0");
  });

  it("keeps the sticky kind header flush with the scroll region top", () => {
    // a block-start padding would leave a strip of moving content above a
    // stuck header; the first folder carries the top gap instead
    const groups = block(".tree-groups");
    expect(groups).not.toContain("padding: 0.5rem");
    expect(groups).not.toContain("padding-top");
    expect(block(".tree-groups > :first-child")).toContain("margin-top: 0.5rem");
    const headers = block(".tree-groups > .tree-folder > .tree-folder__header");
    expect(headers).toContain("top: 0");
  });

  it("keeps the tree status bar fixed under the scroll region", () => {
    const bar = block(".tree-status");
    expect(bar).toContain("flex: none");
    expect(bar).toContain("border-top");
  });

  it("renders the metadata panels without a nested card", () => {
    // the four collapsible cards are gone; each dockview panel is one section,
    // so the meta-panel box (border/background) and the tab strip are gone too
    expect(css.indexOf(".meta-card {")).toBe(-1);
    expect(css.indexOf(".meta-tabs__list {")).toBe(-1);
    expect(block(".meta-panel")).not.toContain("border:");
  });

  it("keeps the state filter and the drift warning on the shared tokens", () => {
    // the tree drift glyph and the metadata-panel block must not invent colours
    const warn = block(".tree-entry__warn");
    expect(warn).toContain("color: var(--warn)");
    const drift = block(".meta-drift");
    expect(drift).toContain("color-mix(in srgb, var(--warn)");
    expect(block(".meta-drift__head")).toContain("color: var(--warn)");
    // the grouped state options reuse the select surface
    expect(block(".ui-select__section-header")).toContain("color: var(--fg-dim)");
  });
});
