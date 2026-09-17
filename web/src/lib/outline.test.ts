import { describe, expect, it } from "vitest";
import { defaultMode, isDocumentMode, isMapPath } from "./documentModes";
import { parseOutline, stripFrontmatter } from "./outline";

describe("documentModes", () => {
  it("defaults map docs to Map and others to Render", () => {
    expect(defaultMode(true)).toBe("map");
    expect(defaultMode(false)).toBe("render");
  });

  it("guards mode values", () => {
    expect(isDocumentMode("code")).toBe(true);
    expect(isDocumentMode("map")).toBe(true);
    expect(isDocumentMode("bogus")).toBe(false);
    expect(isDocumentMode(null)).toBe(false);
  });

  it("detects the .map.md convention", () => {
    expect(isMapPath("context/wiki/topic.map.md")).toBe(true);
    expect(isMapPath("context/wiki/topic.md")).toBe(false);
  });
});

describe("stripFrontmatter", () => {
  it("removes the leading YAML block", () => {
    expect(stripFrontmatter("---\nkind: wiki\n---\n\n# Body\n")).toBe("\n# Body\n");
    expect(stripFrontmatter("# No frontmatter\n")).toBe("# No frontmatter\n");
  });
});

describe("parseOutline", () => {
  it("nests headings and list items", () => {
    const md = `---
kind: wiki
---

# Top

## Section

- item one
  - nested item
- item two

## Second
`;
    const outline = parseOutline(md);
    // the leading `# Top` is the document title and is dropped
    expect(outline.map((i) => i.text)).toEqual(["Section", "Second"]);
    const section = outline[0];
    expect(section.children.map((c) => c.text)).toEqual(["item one", "item two"]);
    expect(section.children[0].children.map((c) => c.text)).toEqual(["nested item"]);
    expect(section.children[0].kind).toBe("list");
  });

  it("drops the leading h1 title from the outline", () => {
    expect(parseOutline("# Title\n\n## Section\n").map((i) => i.text)).toEqual(["Section"]);
  });

  it("ignores fenced code and frontmatter", () => {
    const md = "---\nx: 1\n---\n```\n# not a heading\n- not a list\n```\n# Real\n";
    expect(parseOutline(md).map((i) => i.text)).toEqual(["Real"]);
  });

  it("returns an empty outline for plain prose", () => {
    expect(parseOutline("just a paragraph")).toEqual([]);
  });
});
