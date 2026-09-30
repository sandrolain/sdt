// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import {
  applyFindHighlights,
  clearFindHighlights,
  collectFindRanges,
  DEFAULT_FIND_OPTIONS,
  findMatcher,
} from "./findInDoc";

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

describe("match options", () => {
  it("matches whole words only when asked", () => {
    const root = mount("<p>token tokens tokenizer</p>");
    expect(
      collectFindRanges(root, "token", { ...DEFAULT_FIND_OPTIONS, wholeWord: true }),
    ).toHaveLength(1);
  });

  it("respects case when asked, and ignores it otherwise", () => {
    const root = mount("<p>Foo foo</p>");
    const sensitive = { ...DEFAULT_FIND_OPTIONS, caseSensitive: true };
    expect(collectFindRanges(root, "foo", sensitive)).toHaveLength(1);
    expect(collectFindRanges(root, "Foo", sensitive)).toHaveLength(1);
    expect(collectFindRanges(root, "foo", DEFAULT_FIND_OPTIONS)).toHaveLength(2);
  });

  it("treats the query as a regular expression when asked", () => {
    const root = mount("<p>a1 b22 c333</p>");
    const regex = { ...DEFAULT_FIND_OPTIONS, regex: true };
    expect(collectFindRanges(root, "[a-c]\\d+", regex)).toHaveLength(3);
    // without the option the same query is literal
    expect(collectFindRanges(root, "[a-c]\\d+", DEFAULT_FIND_OPTIONS)).toHaveLength(0);
  });

  it("combines a regular expression with case sensitivity", () => {
    const root = mount("<p>Alpha alpha ALPHA</p>");
    const regexCase = { wholeWord: false, caseSensitive: true, regex: true };
    // only the lowercase occurrence, and it terminates instead of looping
    expect(collectFindRanges(root, "alpha", regexCase)).toHaveLength(1);
    expect(collectFindRanges(root, "ALPHA", regexCase)).toHaveLength(1);
  });

  it("does not loop on a zero-length regular expression match", () => {
    const root = mount("<p>abc</p>");
    const ranges = collectFindRanges(root, "a*", { ...DEFAULT_FIND_OPTIONS, regex: true });
    // only the real match counts; the empty ones are skipped, not looped on
    expect(ranges).toHaveLength(1);
    expect(ranges[0].end - ranges[0].start).toBe(1);
  });

  it("builds no matcher for an empty query", () => {
    expect(findMatcher("   ")).toBeNull();
  });

  it("falls back to the literal query when the expression is invalid", () => {
    const root = mount("<p>a(b</p>");
    const ranges = collectFindRanges(root, "a(b", { ...DEFAULT_FIND_OPTIONS, regex: true });
    expect(ranges).toHaveLength(1);
    expect(ranges[0].end - ranges[0].start).toBe(3);
  });
});
