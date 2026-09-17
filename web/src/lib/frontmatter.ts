/** Structured parsing of a simple YAML frontmatter block for the meta panel. */

import { unwrapQuotes } from "./titles";

export interface FrontmatterField {
  key: string;
  label: string;
  values: string[];
}

/** Human labels for relation verbs (frontmatter `relations`/`part_of`/…). */
const VERB_LABELS: Record<string, string> = {
  part_of: "Part of",
  contains: "Contains",
  depends_on: "Depends on",
  supersedes: "Supersedes",
  replaces: "Replaces",
  derives_from: "Derived from",
  extends: "Extends",
  implements: "Implements",
  related_to: "Related to",
  uses: "Uses",
  references: "References",
  produced_by: "Produced by",
};

/** Translated labels for the known SDT frontmatter keys; unknown keys pass through. */
const LABELS: Record<string, string> = {
  ...VERB_LABELS,
  kind: "Kind",
  id: "Id",
  title: "Title",
  summary: "Summary",
  status: "Status",
  created: "Created",
  created_at: "Created at",
  updated: "Updated",
  type: "Type",
  tags: "Tags",
  relations: "Relations",
  sources: "Sources",
  links: "Links",
  image: "Image",
  context: "Context",
  component: "Component",
  project: "Project",
  group: "Group",
  agent: "Agent",
  model: "Model",
  session: "Session",
};

/** Human label for a frontmatter key. */
export function fieldLabel(key: string): string {
  return LABELS[key] ?? key;
}

/** Relation verbs whose values are document links. */
const RELATION_VERBS = new Set(Object.keys(VERB_LABELS));

/** True when a frontmatter key is a relation verb (its values are links). */
export function isRelationVerb(key: string): boolean {
  return RELATION_VERBS.has(key);
}

/** Human label for a relation verb; unknown verbs become "Title case" words. */
export function verbLabel(verb: string): string {
  if (VERB_LABELS[verb]) return VERB_LABELS[verb];
  const words = verb.replace(/[_-]+/g, " ").trim();
  return words ? words.charAt(0).toUpperCase() + words.slice(1) : verb;
}

/** Strip a single wrapping pair of quotes and trim. */
function scalar(value: string): string {
  return value
    .trim()
    .replace(/^["']|["']$/g, "")
    .trim();
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
 * inline arrays, block sequences (`- item`) and one level of nested maps, so a
 * `relations:` block with a `part_of:` verb becomes a `part_of` field holding
 * its targets. Unknown/malformed lines are skipped; fields that end up with no
 * value (e.g. the now-empty `relations` parent) are dropped. The surrounding
 * `---` delimiters and comments are ignored.
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
    // match on the trimmed line so nested keys (`  part_of:`) are recognised
    const kv = /^([A-Za-z0-9_.-]+):\s*(.*)$/.exec(trimmed);
    if (!kv) continue;
    current = { key: kv[1], label: fieldLabel(kv[1]), values: [] };
    fields.push(current);
    current.values.push(...inlineValues(kv[2]));
  }
  return fields.filter((field) => field.values.length > 0);
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

/** Local date-only for a frontmatter date value; raw value when unparseable. */
export function formatFieldDateOnly(value: string, locale?: string): string {
  const date = parseFieldDate(value);
  if (!date) return value;
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(date);
}
