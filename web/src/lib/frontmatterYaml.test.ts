// @vitest-environment node
import { beforeEach, describe, expect, it } from "vitest";
import { clearFrontmatterCache, loadFrontmatter, type FrontmatterParse } from "./frontmatterYaml";

function fieldsOf(result: FrontmatterParse) {
  if ("error" in result) throw new Error(`unexpected parse error: ${result.error}`);
  return result.fields;
}

beforeEach(() => {
  clearFrontmatterCache();
});

describe("loadFrontmatter", () => {
  it("parses scalars, inline arrays and block sequences into rows", async () => {
    const fm = [
      "---",
      "kind: analysis",
      "title: Quoted Title",
      "tags: [alpha, beta]",
      "sources:",
      "  - analysis/a.md",
      "  - plan/b.md",
      "---",
    ].join("\n");
    const fields = fieldsOf(await loadFrontmatter(fm));
    expect(fields.map((f) => f.key)).toEqual(["kind", "title", "tags", "sources"]);
    expect(fields[1].values).toEqual(["Quoted Title"]);
    expect(fields[2].values).toEqual(["alpha", "beta"]);
    expect(fields[3].values).toEqual(["analysis/a.md", "plan/b.md"]);
    expect(fields[3].path).toEqual(["sources"]);
  });

  it("collapses a block scalar instead of showing the marker", async () => {
    const fields = fieldsOf(await loadFrontmatter("---\nsummary: >\n  a long\n  summary\n---\n"));
    expect(fields).toEqual([
      { path: ["summary"], key: "summary", label: "Summary", values: ["a long summary\n"] },
    ]);
  });

  it("keeps the parent path as the label for a nested map", async () => {
    const fields = fieldsOf(await loadFrontmatter("---\nmarkmap:\n  colorFreezeLevel: 2\n---\n"));
    expect(fields).toEqual([
      {
        path: ["markmap", "colorFreezeLevel"],
        key: "colorFreezeLevel",
        label: "Markmap · Color Freeze Level",
        values: ["2"],
      },
    ]);
  });

  it("labels a nested relation verb with its verb label", async () => {
    const fm = ["---", "relations:", "  part_of:", '    - "[[b|Beta]]"', "---"].join("\n");
    const fields = fieldsOf(await loadFrontmatter(fm));
    expect(fields).toEqual([
      {
        path: ["relations", "part_of"],
        key: "part_of",
        label: "Part of",
        values: ["[[b|Beta]]"],
      },
    ]);
  });

  it("renders a list of maps as one row per item", async () => {
    const fm = "---\nitems:\n  - name: a\n    value: 1\n  - name: b\n---\n";
    const fields = fieldsOf(await loadFrontmatter(fm));
    expect(fields).toHaveLength(2);
    expect(fields[0].values).toEqual(['{"name":"a","value":1}']);
    expect(fields[1].values).toEqual(['{"name":"b"}']);
  });

  it("drops an empty container but keeps an explicit empty scalar", async () => {
    const fields = fieldsOf(await loadFrontmatter("---\nrelations: {}\nimage:\n---\n"));
    expect(fields).toEqual([{ path: ["image"], key: "image", label: "Image", values: [""] }]);
  });

  it("returns an error for malformed frontmatter instead of dropping keys", async () => {
    const result = await loadFrontmatter("---\nderived_from:\n\t- a\n---\n");
    expect(result).toHaveProperty("error");
  });

  it("returns an error when the block is not a mapping", async () => {
    const result = await loadFrontmatter("---\nstatus active\n---\n");
    expect(result).toHaveProperty("error");
  });

  it("returns no fields for an empty block", async () => {
    expect(await loadFrontmatter("---\n---\n")).toEqual({ fields: [] });
  });

  it("caches by the raw block", () => {
    const a = loadFrontmatter("---\nkind: x\n---\n");
    const b = loadFrontmatter("---\nkind: x\n---\n");
    expect(a).toBe(b);
  });
});
