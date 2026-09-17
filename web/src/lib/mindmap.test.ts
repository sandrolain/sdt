import { describe, expect, it } from "vitest";
import { markmapMarkdown, transformMindmap } from "./mindmap";
import { buildWikiIndex } from "./wikiLinks";

const INDEX = buildWikiIndex([{ path: "context/wiki/accounts.md", title: "Account Service" }]);

describe("markmapMarkdown", () => {
  it("strips frontmatter and rewrites both link syntaxes", () => {
    const md = [
      "---",
      "kind: wiki",
      "---",
      "# Root",
      "",
      "- see [[accounts|Account Service]]",
      "- and [[refers_to::Account Service]]",
      "- doc [other](../notes/other.md)",
    ].join("\n");
    const out = markmapMarkdown(md, { basePath: "context/wiki/root.md", wikiIndex: INDEX });
    expect(out).not.toContain("kind: wiki");
    expect(out).toContain("[Account Service](/wiki/accounts)");
    expect(out).toContain("[Account Service](/wiki/accounts)");
    expect(out).toContain("[other](/docs/context/notes/other.md)");
  });
});

describe("transformMindmap", () => {
  it("produces a tree from headings and lists with resolved links", () => {
    const md = "# Root\n\n- child one\n- [[accounts|Account Service]]\n  - nested\n";
    const root = transformMindmap(md, { basePath: "context/wiki/root.md", wikiIndex: INDEX });
    expect(root.children.length).toBeGreaterThan(0);
    const flat = JSON.stringify(root);
    expect(flat).toContain("/wiki/accounts");
    expect(flat).toContain("nested");
  });

  it("does not mangle verb-form wikilinks", () => {
    const md = "- [[depends_on::Account Service]]\n";
    const root = transformMindmap(md, { basePath: "context/wiki/root.md", wikiIndex: INDEX });
    expect(JSON.stringify(root)).toContain("/wiki/accounts");
    expect(JSON.stringify(root)).not.toContain("depends_on::");
  });
});
