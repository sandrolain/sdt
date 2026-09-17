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

  it("resolves body images through /api/file and keeps remote srcs", () => {
    const html = renderMarkdown(
      [
        "![Cover](context/assets/cover.png)",
        "![Rel](../assets/photo.jpg)",
        "![Remote](https://example.com/a.png)",
        '![Titled](context/assets/x.jpg "A caption")',
      ].join("\n\n"),
      { basePath: "context/notes/n.md" },
    );
    expect(html).toContain('src="/api/file?path=context%2Fassets%2Fcover.png"');
    expect(html).toContain('src="/api/file?path=context%2Fassets%2Fphoto.jpg"');
    expect(html).toContain('src="https://example.com/a.png"');
    expect(html).toContain("<figure>");
    expect(html).toContain("<figcaption>A caption</figcaption>");
  });

  it("turns math into KaTeX placeholders outside code blocks", () => {
    const html = renderMarkdown(
      [
        "Inline $a^2 + b^2 = c^2$ math.",
        "",
        "$$",
        "\\int_0^1 x\\,dx = \\frac{1}{2}",
        "$$",
        "",
        "```js",
        'const price = "$5";',
        "```",
      ].join("\n"),
    );
    expect(html).toContain('class="md-math"');
    expect(html).toContain('data-tex="a^2 + b^2 = c^2"');
    expect(html).toContain('data-display="block"');
    // `$` inside a fenced code block is left alone
    expect(html).toContain("$5");
    expect(html).not.toContain('data-tex="5"');
  });

  it("adds a language badge and highlighted code lines", () => {
    const html = renderMarkdown("```js {2}\nconst a = 1;\nconst b = 2;\n```\n");
    expect(html).toContain('<span class="md-code__lang">js</span>');
    expect(html).toContain("md-code__wrap");
    expect(html).toMatch(/md-code__line is-hl/);
  });

  it("wraps tables for scrolling with alignment classes", () => {
    const html = renderMarkdown("| a | b | c |\n| :-- | :-: | --: |\n| 1 | 2 | 3 |\n");
    expect(html).toContain('class="md-table-wrap"');
    expect(html).toContain('class="md-align-left"');
    expect(html).toContain('class="md-align-center"');
    expect(html).toContain('class="md-align-right"');
  });

  it("renders footnotes and definition lists", () => {
    const html = renderMarkdown(
      [
        "Text with a note.[^1]",
        "",
        "[^1]: The note body.",
        "",
        "Term",
        ": Definition one",
        ": Definition two",
      ].join("\n"),
    );
    expect(html).toContain("The note body.");
    expect(html).toMatch(/class="[^"]*footnotes[^"]*"/);
    expect(html).toContain('class="md-deflist"');
    expect(html).toContain("<dt>Term</dt>");
    expect(html).toContain("<dd>Definition one</dd>");
    expect(html).toContain("<dd>Definition two</dd>");
  });

  it("renders GitHub-style callouts", () => {
    const html = renderMarkdown("> [!NOTE]\n> Useful information.\n");
    expect(html).toContain('class="md-callout md-callout--note"');
    expect(html).toContain("md-callout__title");
    expect(html).toContain("Useful information.");
    // a plain blockquote is untouched
    expect(renderMarkdown("> just a quote\n")).toContain("<blockquote>");
  });

  it("gives headings slug ids and a copy-link anchor", () => {
    const html = renderMarkdown("## Hello World!\n\n## Hello World!\n");
    expect(html).toContain('<h2 id="hello-world" data-heading="Hello World!">');
    expect(html).toContain('<h2 id="hello-world-1" data-heading="Hello World!">');
    expect(html).toContain('class="md-anchor"');
    expect(html).toContain('data-anchor="hello-world"');
  });

  it("turns mermaid fences into render placeholders", () => {
    const html = renderMarkdown("```mermaid\nflowchart LR\n  A --> B\n```\n");
    expect(html).toContain('class="md-mermaid"');
    expect(html).toContain(`data-src="${encodeURIComponent("flowchart LR\n  A --> B")}"`);
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
