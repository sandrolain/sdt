// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  booleanValue,
  fieldLabel,
  formatFieldDate,
  formatFieldDateOnly,
  isRelationVerb,
  parseFieldDate,
  parseFrontmatter,
  verbLabel,
} from "./frontmatter";

describe("parseFrontmatter", () => {
  it("parses scalars, inline arrays and block sequences", () => {
    const fm = [
      "---",
      "kind: analysis",
      'title: "Quoted Title"',
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

  it("parses nested relation verbs into their own labelled field", () => {
    const fm = [
      "---",
      "kind: wiki",
      "relations:",
      "  part_of:",
      '    - "[[sdt-context-memory|SDT context memory]]"',
      "tags: [a]",
      "---",
    ].join("\n");
    const fields = parseFrontmatter(fm);
    expect(fields.map((f) => f.key)).toEqual(["kind", "part_of", "tags"]);
    const partOf = fields.find((f) => f.key === "part_of");
    expect(partOf?.label).toBe("Part of");
    expect(partOf?.values).toEqual(["[[sdt-context-memory|SDT context memory]]"]);
    expect(isRelationVerb("part_of")).toBe(true);
    expect(isRelationVerb("kind")).toBe(false);
  });

  it("humanises relation verbs", () => {
    expect(verbLabel("part_of")).toBe("Part of");
    expect(verbLabel("depends_on")).toBe("Depends on");
    expect(verbLabel("custom_verb")).toBe("Custom verb");
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

  it("parses quoted ISO values shipped by the API", () => {
    const quoted = '"2026-09-15T10:30:00Z"';
    expect(parseFieldDate(quoted)).not.toBeNull();
    expect(formatFieldDate(quoted, "en-US")).toContain("2026");
  });

  it("includes the time in the formatted output", () => {
    const formatted = formatFieldDate("2026-09-15T14:45:00Z", "en-GB");
    expect(formatted).toMatch(/\d{2}:\d{2}/);
  });

  it("formats a date-only variant without the time", () => {
    const formatted = formatFieldDateOnly("2026-09-15T14:45:00Z", "en-GB");
    expect(formatted).toContain("2026");
    expect(formatted).not.toMatch(/\d{2}:\d{2}/);
    expect(formatFieldDateOnly("not-a-date")).toBe("not-a-date");
  });
});
