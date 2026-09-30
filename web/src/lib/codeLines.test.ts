// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import {
  codeBlockText,
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

  it("joins the line wrappers without a newline, so <pre> renders one line each", () => {
    const html = wrapHighlightedLines("a\nb\nc", new Set());
    expect(html).toBe(
      '<span class="md-code__line">a</span><span class="md-code__line">b</span><span class="md-code__line">c</span>',
    );
    expect(html).not.toContain("</span>\n");
  });
});

describe("codeBlockText", () => {
  function codeEl(html: string): HTMLElement {
    const code = document.createElement("code");
    code.innerHTML = html;
    return code;
  }

  it("re-joins the per-line wrappers with newlines", () => {
    expect(codeBlockText(codeEl(wrapHighlightedLines("a\nb", new Set([1]))))).toBe("a\nb");
  });

  it("keeps a single-line block verbatim", () => {
    expect(codeBlockText(codeEl(wrapHighlightedLines("const x = 1;", new Set())))).toBe(
      "const x = 1;",
    );
  });

  it("falls back to textContent when there are no line wrappers", () => {
    expect(codeBlockText(codeEl("plain <b>code</b>"))).toBe("plain code");
  });

  it("returns an empty string for a missing element", () => {
    expect(codeBlockText(null)).toBe("");
  });
});
