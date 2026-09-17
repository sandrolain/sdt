// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  lineNumbers,
  parseCodeInfo,
  splitHighlightedLines,
  wrapHighlightedLines,
} from "./codeLines";

describe("lineNumbers", () => {
  it("emits one number per source line", () => {
    expect(lineNumbers("a\nb\nc")).toBe("1\n2\n3");
    expect(lineNumbers("single")).toBe("1");
    expect(lineNumbers("trailing\n")).toBe("1\n2");
  });
});

describe("parseCodeInfo", () => {
  it("splits the language from the highlighted line spec", () => {
    expect(parseCodeInfo("js {1,3-5}")).toEqual({
      language: "js",
      highlighted: new Set([1, 3, 4, 5]),
    });
    expect(parseCodeInfo("go")).toEqual({ language: "go", highlighted: new Set() });
    expect(parseCodeInfo(undefined)).toEqual({ language: "", highlighted: new Set() });
  });
});

describe("splitHighlightedLines", () => {
  it("balances spans across line breaks", () => {
    const lines = splitHighlightedLines('<span class="hljs-comment">a\nb</span>');
    expect(lines[0]).toBe('<span class="hljs-comment">a</span>');
    expect(lines[1]).toBe('<span class="hljs-comment">b</span>');
  });
});

describe("wrapHighlightedLines", () => {
  it("wraps each line and marks the highlighted ones", () => {
    const html = wrapHighlightedLines("a\nb", new Set([2]));
    expect(html).toContain('<span class="md-code__line">a</span>');
    expect(html).toContain('<span class="md-code__line is-hl">b</span>');
  });
});
