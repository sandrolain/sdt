import { describe, expect, it } from "vitest";
import { parseMapDocument, type MindNode, type MapNodeKind } from "./mindmap";
import { buildWikiIndex } from "./wikiLinks";

const INDEX = buildWikiIndex([{ path: "context/wiki/accounts.md", title: "Account Service" }]);

/** Flat `kind: text` view of the tree, one line per node in document order. */
function outline(node: MindNode, depth = 0): string[] {
  const kind = node.payload?.kind ?? "topic";
  const label = node.content
    .replace(/<br>/g, " / ")
    .replace(/<[^>]*>/g, "")
    .trim();
  return [
    `${"  ".repeat(depth)}${kind}: ${label}`,
    ...node.children.flatMap((c) => outline(c, depth + 1)),
  ];
}

function kinds(node: MindNode): MapNodeKind[] {
  return [node.payload?.kind ?? "topic", ...node.children.flatMap(kinds)];
}

describe("parseMapDocument", () => {
  it("attaches a list to the innermost open heading (the markmap rule)", () => {
    const root = parseMapDocument("# Root\n\n## Branch\n\n### Leaf\n\n- child of the leaf\n");
    expect(outline(root)).toEqual([
      "topic: Root",
      "  topic: Branch",
      "    topic: Leaf",
      "      topic: child of the leaf",
    ]);
  });

  it("keeps list nesting and an item's second paragraph as a child", () => {
    const root = parseMapDocument(
      ["# Root", "", "- one", "", "  extra prose", "", "  - deep", ""].join("\n"),
    );
    expect(outline(root)).toEqual([
      "topic: Root",
      "  topic: one",
      "    topic: extra prose",
      "    topic: deep",
    ]);
  });

  it("attaches a loose document list to the enclosing heading", () => {
    const root = parseMapDocument("# Root\n\n## Branch\n\n- a\n- b\n");
    expect(outline(root)).toEqual([
      "topic: Root",
      "  topic: Branch",
      "    topic: a",
      "    topic: b",
    ]);
  });

  it("falls back to the frontmatter title when there is no H1", () => {
    const md = "---\ntitle: From Frontmatter\nkind: notes\n---\n\n## Branch\n";
    const root = parseMapDocument(md);
    expect(root.content).toBe("From Frontmatter");
    expect(outline(root)).toEqual([
      "topic: Root".replace("Root", "From Frontmatter"),
      "  topic: Branch",
    ]);
  });

  it("makes a later H1 a child of the root instead of losing it", () => {
    const root = parseMapDocument("# Root\n\n# Second\n\n- x\n");
    expect(root.content).toBe("Root");
    expect(outline(root)).toEqual(["topic: Root", "  topic: Second", "    topic: x"]);
  });

  it("keeps paragraphs, blockquotes, code and tables as nodes", () => {
    const md = [
      "# Root",
      "",
      "A prose paragraph.",
      "",
      "> quoted line",
      ">",
      "> second quoted line",
      "",
      "```ts",
      "const a = 1;",
      "```",
      "",
      "| h1 | h2 |",
      "| -- | -- |",
      "| a | b |",
      "",
    ].join("\n");
    const root = parseMapDocument(md);
    expect(kinds(root)).toEqual(["topic", "topic", "quote", "code", "table"]);
    expect(outline(root)).toEqual([
      "topic: Root",
      "  topic: A prose paragraph.",
      "  quote: quoted line / second quoted line",
      "  code: const a = 1;",
      "  table: h1 · h2 / a · b",
    ]);
  });

  it("keeps a blockquote's nested list as the quote's children", () => {
    const root = parseMapDocument("# Root\n\n> quoted\n>\n> - a\n> - b\n");
    expect(outline(root)).toEqual([
      "topic: Root",
      "  quote: quoted",
      "    topic: a",
      "    topic: b",
    ]);
  });

  it("never drops an unrecognised block: raw HTML stays escaped text", () => {
    const root = parseMapDocument('# Root\n\n<div onclick="steal()">x</div>\n');
    const leaf = root.children[0];
    expect(leaf.payload?.kind).toBe("text");
    expect(leaf.content).not.toContain("<div");
    expect(leaf.content).toContain("&lt;div");
  });

  it("rewrites wikilinks and doc links into ready-to-render HTML", () => {
    const md = [
      "---",
      "kind: notes",
      "---",
      "# Root",
      "",
      "- see [[accounts|Account Service]]",
      "- doc [other](../notes/other.md)",
    ].join("\n");
    const root = parseMapDocument(md, { basePath: "context/wiki/root.md", wikiIndex: INDEX });
    expect(JSON.stringify(root)).not.toContain("kind: notes");
    expect(root.children[0].content).toBe('see <a href="/wiki/accounts">Account Service</a>');
    expect(root.children[1].content).toBe('doc <a href="/docs/context/notes/other.md">other</a>');
  });

  it("does not mangle verb-form wikilinks", () => {
    const root = parseMapDocument("- [[depends_on::Account Service]]\n", {
      basePath: "context/wiki/root.md",
      wikiIndex: INDEX,
    });
    const flat = JSON.stringify(root);
    expect(flat).toContain("/wiki/accounts");
    expect(flat).not.toContain("depends_on::");
  });

  it("renders inline formatting and a task checkbox into the label", () => {
    const root = parseMapDocument("# Root\n\n- **bold** and *em*\n- [ ] todo\n- [x] done\n");
    expect(root.children[0].content).toContain("<strong>bold</strong>");
    expect(outline(root).slice(1)).toEqual([
      "  topic: bold and em",
      "  topic: [ ] todo",
      "  topic: [x] done",
    ]);
  });

  it("keeps a fused-map reference discoverable through the label HTML", async () => {
    const { extractMapRefs } = await import("./fuse");
    const md = "# Root\n\n- [Other](/wiki/other.map)\n";
    const root = parseMapDocument(md);
    expect(extractMapRefs(root, new Map([["other.map", { id: "other.map" }]]))).toEqual([
      "other.map",
    ]);
  });
});
