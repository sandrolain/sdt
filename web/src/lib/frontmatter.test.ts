// @vitest-environment node
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  booleanValue,
  fieldLabel,
  formatFieldDate,
  formatFieldDateOnly,
  isRelationVerb,
  parseFieldDate,
  parseFrontmatter,
  STATUS_VALUES,
  valueLabel,
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

  it("labels known keys and title-cases unknown keys", () => {
    expect(fieldLabel("created")).toBe("Created");
    expect(fieldLabel("objective")).toBe("Objective");
    expect(fieldLabel("topics")).toBe("Topics");
    expect(fieldLabel("custom")).toBe("Custom");
    expect(fieldLabel("some_custom_key")).toBe("Some custom key");
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

describe("valueLabel", () => {
  it("resolves known closed-vocabulary values", () => {
    expect(valueLabel("analysis", "archived")?.tone).toBe("neutral");
    expect(valueLabel("plan", "completed")?.label).toBe("Completed");
    expect(valueLabel("questions", "active")?.tone).toBe("danger");
  });

  it("normalises case and whitespace", () => {
    expect(valueLabel("analysis", " ARCHIVED ")?.tone).toBe("neutral");
  });

  it("returns null for an unknown value or kind", () => {
    expect(valueLabel("analysis", "completed")).toBeNull();
    expect(valueLabel("nope", "active")).toBeNull();
  });
});

describe("status vocabulary drift guard", () => {
  // The status matrix in context/architecture/stack.md is the prose source of
  // truth; this guards the hand-maintained frontend table against drift.
  const matrix = readFileSync(
    new URL("../../../context/architecture/stack.md", import.meta.url),
    "utf8",
  );

  /** Parse the `| kind | default | \`a\` · \`b\` | meaning |` rows. */
  function matrixVocab(): Map<string, string[]> {
    const rows = new Map<string, string[]>();
    for (const line of matrix.split("\n")) {
      const m = /^\|\s*([a-z]+)\s*\|[^|]*\|([^|]*)\|/.exec(line);
      if (!m) continue;
      const statuses = [...m[2].matchAll(/`([a-z-]+)`/g)].map((x) => x[1]);
      // `reference` describes fan-in sources outside the registry; excluded.
      if (m[1] === "reference" || statuses.length === 0) continue;
      rows.set(m[1], statuses);
    }
    return rows;
  }

  it("matches the frontend table per kind", () => {
    const rows = matrixVocab();
    expect(rows.size).toBeGreaterThan(5);
    for (const [kind, statuses] of rows) {
      const frontend = Object.keys(STATUS_VALUES)
        .filter((key) => key.startsWith(`${kind}.`))
        .map((key) => key.slice(kind.length + 1));
      expect(frontend.sort(), `kind ${kind}`).toEqual([...statuses].sort());
    }
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
