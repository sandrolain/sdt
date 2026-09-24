// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { applyFindHighlights, clearFindHighlights, collectFindRanges } from "./findInDoc";

function mount(html: string): HTMLElement {
  const div = document.createElement("div");
  div.innerHTML = html;
  return div;
}

afterEach(() => {
  document.body.innerHTML = "";
});

describe("collectFindRanges", () => {
  it("collects every case-insensitive match across text nodes", () => {
    const root = mount("<p>Foo bar</p><p>foo baz</p>");
    const ranges = collectFindRanges(root, "foo");
    expect(ranges).toHaveLength(2);
    expect(ranges.map((r) => r.index)).toEqual([0, 1]);
    expect(ranges[0].node.data).toBe("Foo bar");
    expect(ranges[0].start).toBe(0);
    expect(ranges[1].node.data).toBe("foo baz");
  });

  it("finds multiple matches inside a single text node", () => {
    const root = mount("<p>tokens and tokens</p>");
    const ranges = collectFindRanges(root, "tokens");
    expect(ranges).toHaveLength(2);
    expect(ranges[0].start).toBe(0);
    expect(ranges[1].start).toBe(11);
  });

  it("returns nothing for an empty or whitespace query", () => {
    const root = mount("<p>tokens</p>");
    expect(collectFindRanges(root, "")).toHaveLength(0);
    expect(collectFindRanges(root, "   ")).toHaveLength(0);
  });

  it("skips svg, math and mermaid subtrees", () => {
    const root = mount(
      "<p>tokens text</p>" +
        "<svg><text>tokens svg</text><g><tspan>tokens nested</tspan></g></svg>" +
        '<div class="md-math">tokens math</div>' +
        '<div class="md-mermaid"><pre>tokens mermaid</pre></div>',
    );
    const ranges = collectFindRanges(root, "tokens");
    expect(ranges).toHaveLength(1);
    expect(ranges[0].node.data).toBe("tokens text");
  });
});

describe("applyFindHighlights", () => {
  it("wraps matches in marks and flags the current one", () => {
    const root = mount("<p>tokens and tokens</p>");
    applyFindHighlights(root, collectFindRanges(root, "tokens"), 1);
    const marks = root.querySelectorAll("mark.find-hit");
    expect(marks).toHaveLength(2);
    expect(marks[0].classList.contains("is-current")).toBe(false);
    expect(marks[1].classList.contains("is-current")).toBe(true);
    expect(root.textContent).toBe("tokens and tokens");
  });

  it("clears previous highlights before applying new ones", () => {
    const root = mount("<p>tokens tokens</p>");
    applyFindHighlights(root, collectFindRanges(root, "tokens"), 0);
    expect(root.querySelectorAll("mark.find-hit")).toHaveLength(2);
    applyFindHighlights(root, collectFindRanges(root, "none"), 0);
    expect(root.querySelectorAll("mark.find-hit")).toHaveLength(0);
    expect(root.textContent).toBe("tokens tokens");
  });

  it("restores the original text after clearing", () => {
    const root = mount("<p>tokens <em>tokens</em> end</p>");
    applyFindHighlights(root, collectFindRanges(root, "tokens"), 0);
    clearFindHighlights(root);
    expect(root.querySelectorAll("mark.find-hit")).toHaveLength(0);
    expect(root.textContent).toBe("tokens tokens end");
    expect(root.innerHTML).toBe("<p>tokens <em>tokens</em> end</p>");
  });
});
