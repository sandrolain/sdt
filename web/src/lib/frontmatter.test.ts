// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  booleanValue,
  fieldLabel,
  formatFieldDate,
  parseFieldDate,
  parseFrontmatter,
} from "./frontmatter";

describe("parseFrontmatter", () => {
  it("parses scalars, inline arrays and block sequences", () => {
    const fm = [
      "---",
      "kind: analysis",
      "title: \"Quoted Title\"",
      "tags: [alpha, beta]",
      "sources:",
      "  - analysis/a.md",
      "  - plan/b.md",
      "# comment",
      "---",
    ].join("\n");
    const fields = parseFrontmatter(fm);
    expect(fields.map((f) => f.key)).toEqual(["kind", "title", "tags", "sources"]);
    expect(fields[0].values).toEqual(["analysis"]);
    expect(fields[1].values).toEqual(["Quoted Title"]);
    expect(fields[2].values).toEqual(["alpha", "beta"]);
    expect(fields[3].values).toEqual(["analysis/a.md", "plan/b.md"]);
  });

  it("labels known keys and passes unknown keys through", () => {
    expect(fieldLabel("created")).toBe("Created");
    expect(fieldLabel("custom")).toBe("custom");
  });

  it("returns an empty list without frontmatter", () => {
    expect(parseFrontmatter(undefined)).toEqual([]);
    expect(parseFrontmatter("")).toEqual([]);
  });
});

describe("booleanValue", () => {
  it("recognises boolean spellings", () => {
    expect(booleanValue("true")).toBe(true);
    expect(booleanValue("no")).toBe(false);
    expect(booleanValue("maybe")).toBeNull();
  });
});

describe("dates", () => {
  it("parses date-only values without timezone drift", () => {
    const d = parseFieldDate("2026-09-15");
    expect(d?.getFullYear()).toBe(2026);
    expect(d?.getMonth()).toBe(8);
    expect(d?.getDate()).toBe(15);
  });

  it("formats a date to a non-empty local string", () => {
    expect(formatFieldDate("2026-09-15", "en-US")).toContain("2026");
    expect(formatFieldDate("not-a-date")).toBe("not-a-date");
  });
});
