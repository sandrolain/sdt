// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { highlightCode, highlightMarkdown, renderMarkdown, stripBoundaryMarkers } from "./markdown";
import { buildWikiIndex } from "./wikiLinks";

const INDEX = buildWikiIndex([{ path: "context/wiki/accounts.md", title: "Account Service" }]);

describe("stripBoundaryMarkers", () => {
  it("removes inline markers and boundary title lines", () => {
    const md = "- topic one [B1]\n- topic two\n  [B1]: Group title\n";
    expect(stripBoundaryMarkers(md)).toBe("- topic one \n- topic two\n");
  });

  it("removes unnumbered markers", () => {
    expect(stripBoundaryMarkers("a [B] b\n")).toBe("a  b\n");
  });
});

describe("renderMarkdown", () => {
  it("renders standard markdown without the leading title h1", () => {
    const html = renderMarkdown("# Title\n\nHello **world**.");
    expect(html).not.toContain("<h1>");
    expect(html).toContain("<strong>world</strong>");
  });

  it("keeps real headings and adds a copy button to code blocks", () => {
    const html = renderMarkdown("# Title\n\n## Section\n\n```js\nconst x = 1;\n```");
    expect(html).not.toContain("<h1>");
    expect(html).toContain(">Section<");
    expect(html).toContain("md-code__copy");
    expect(html).toContain("content_copy");
  });

  it("sanitizes raw HTML: strips scripts and event handlers, keeps safe tags", () => {
    const html = renderMarkdown(
      '<script>alert(1)</script>\n\n<details open><summary>More</summary>x</details>\n\n<img src=x onerror="alert(1)">',
    );
    expect(html).not.toContain("<script");
    expect(html).not.toContain("onerror");
    expect(html).toContain("<details>");
    expect(html).toContain("<summary>More</summary>");
    expect(html).toContain("<img");
  });

  it("rewrites wikilinks to wiki routes and .md links to docs routes", () => {
    const html = renderMarkdown("[[accounts|Account Service]] and [other](../notes/other.md)", {
      basePath: "context/wiki/accounts.md",
      wikiIndex: INDEX,
    });
    expect(html).toContain('href="/wiki/accounts"');
    expect(html).toContain('href="/docs/context/notes/other.md"');
  });

  it("hardens external links with target and rel", () => {
    const html = renderMarkdown("[site](https://example.com)");
    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noopener noreferrer"');
  });

  it("strips boundary markers before rendering", () => {
    const html = renderMarkdown("- topic [B1]\n  [B1]: Hidden group\n");
    expect(html).not.toContain("[B1]");
    expect(html).not.toContain("Hidden group");
  });

  it("highlights fenced code", () => {
    const html = renderMarkdown("```js\nconst x = 1;\n```");
    expect(html).toContain('class="md-code"');
    expect(html).toContain('class="hljs');
  });
});

describe("highlightCode / highlightMarkdown", () => {
  it("highlights known languages and falls back to plaintext", () => {
    expect(highlightCode("const x = 1;", "js")).toContain("hljs");
    expect(highlightCode("just text", "not-a-language")).toContain("just text");
    expect(highlightMarkdown("# Title")).toContain("hljs");
  });
});
