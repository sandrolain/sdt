/** Structured parsing of a simple YAML frontmatter block for the meta panel. */

import { unwrapQuotes } from "./titles";

export interface FrontmatterField {
  key: string;
  label: string;
  values: string[];
}

/** Translated labels for the known SDT frontmatter keys; unknown keys pass through. */
const LABELS: Record<string, string> = {
  kind: "Kind",
  title: "Title",
  summary: "Summary",
  status: "Status",
  created: "Created",
  updated: "Updated",
  type: "Type",
  tags: "Tags",
  relations: "Relations",
  sources: "Sources",
  links: "Links",
};

/** Human label for a frontmatter key. */
export function fieldLabel(key: string): string {
  return LABELS[key] ?? key;
}

/** Strip a single wrapping pair of quotes and trim. */
function scalar(value: string): string {
  return value.trim().replace(/^["']|["']$/g, "").trim();
}

/** Values from an inline scalar or `[a, b]` array. */
function inlineValues(value: string): string[] {
  if (value.startsWith("[") && value.endsWith("]")) {
    return value
      .slice(1, -1)
      .split(",")
      .map((v) => scalar(v))
      .filter((v) => v !== "");
  }
  const one = scalar(value);
  return one === "" ? [] : [one];
}

/**
 * Parse the frontmatter body into ordered fields. Supports `key: value`,
 * inline arrays and block sequences (`- item`); unknown/malformed lines are
 * skipped. The surrounding `---` delimiters and comments are ignored.
 */
export function parseFrontmatter(frontmatter?: string): FrontmatterField[] {
  if (!frontmatter) return [];
  const fields: FrontmatterField[] = [];
  let current: FrontmatterField | null = null;
  for (const rawLine of frontmatter.split(/\r?\n/)) {
    const line = rawLine.replace(/\s+$/, "");
    const trimmed = line.trim();
    if (trimmed === "" || trimmed === "---" || trimmed.startsWith("#")) continue;
    const seq = /^\s*-\s+(.*)$/.exec(line);
    if (seq && current) {
      const value = scalar(seq[1]);
      if (value !== "") current.values.push(value);
      continue;
    }
    const kv = /^([A-Za-z0-9_.-]+):\s*(.*)$/.exec(line);
    if (!kv) continue;
    current = { key: kv[1], label: fieldLabel(kv[1]), values: [] };
    fields.push(current);
    current.values.push(...inlineValues(kv[2]));
  }
  return fields;
}

/** Boolean field rendered as a check/close icon when the value is true/false. */
export function booleanValue(value: string): boolean | null {
  const v = value.trim().toLowerCase();
  if (v === "true" || v === "yes") return true;
  if (v === "false" || v === "no") return false;
  return null;
}

/** Parse a frontmatter date value; null when unparseable. Quotes are unwrapped. */
export function parseFieldDate(value: string): Date | null {
  const trimmed = unwrapQuotes(value);
  if (trimmed === "") return null;
  const dateOnly = /^(\d{4})-(\d{2})-(\d{2})$/.exec(trimmed);
  if (dateOnly) {
    const d = new Date(Number(dateOnly[1]), Number(dateOnly[2]) - 1, Number(dateOnly[3]));
    return Number.isNaN(d.getTime()) ? null : d;
  }
  const date = new Date(trimmed);
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local date+time for a frontmatter date value; raw value when unparseable. */
export function formatFieldDate(value: string, locale?: string): string {
  const date = parseFieldDate(value);
  if (!date) return value;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}
