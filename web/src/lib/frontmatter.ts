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

/** Translated labels for the known SDT frontmatter keys. */
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
  topics: "Topics",
  objective: "Objective",
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
  role: "Role",
  number: "Number",
  slug: "Slug",
  tasks: "Tasks",
  procedure: "Procedure",
  subject: "Subject",
  markmap: "Markmap",
};

/** Title-case an unknown key so it never renders as a raw identifier. */
function titleCase(key: string): string {
  const words = key.replace(/[_-]+/g, " ").trim();
  return words ? words.charAt(0).toUpperCase() + words.slice(1) : key;
}

/** Human label for a frontmatter key; unknown keys become "Title case". */
export function fieldLabel(key: string): string {
  return LABELS[key] ?? titleCase(key);
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

/** Tone vocabulary shared with the status-dot model (see statusDot.ts). */
export type ValueTone = "danger" | "warn" | "ok" | "neutral";

export interface ValueLabel {
  label: string;
  /** the meaning written in the status matrix (architecture/stack.md) */
  meaning: string;
  tone: ValueTone;
}

/**
 * Closed-vocabulary value labels, keyed `<kind>.<status>`, transcribed from the
 * status matrix in `context/architecture/stack.md`. Open vocabularies (tags,
 * topics, custom keys) are intentionally absent and pass through unchanged.
 * The drift-guard test asserts the per-kind status sets match stack.md.
 */
export const STATUS_VALUES: Record<string, ValueLabel> = {
  "plan.active": {
    label: "Active",
    meaning: "Open and being executed",
    tone: "warn",
  },
  "plan.completed": { label: "Completed", meaning: "Deliverable done", tone: "ok" },
  "plan.abandoned": { label: "Abandoned", meaning: "Stopped", tone: "neutral" },
  "analysis.active": {
    label: "Active",
    meaning: "Concluded, recommendation ready",
    tone: "ok",
  },
  "analysis.draft": {
    label: "Draft",
    meaning: "Started, not yet concluded",
    tone: "warn",
  },
  "analysis.archived": {
    label: "Archived",
    meaning: "Superseded and closed",
    tone: "neutral",
  },
  "tasks.pending": { label: "Pending", meaning: "Not started", tone: "danger" },
  "tasks.in-progress": { label: "In progress", meaning: "Being worked", tone: "warn" },
  "tasks.completed": { label: "Completed", meaning: "Done", tone: "ok" },
  "tasks.archived": { label: "Archived", meaning: "Closed", tone: "neutral" },
  "tasks.active": {
    label: "Active",
    meaning: "Legacy value still accepted",
    tone: "warn",
  },
  "decision.proposed": { label: "Proposed", meaning: "Staged", tone: "warn" },
  "decision.accepted": { label: "Accepted", meaning: "Decided for", tone: "ok" },
  "decision.rejected": { label: "Rejected", meaning: "Decided against", tone: "danger" },
  "decision.deprecated": {
    label: "Deprecated",
    meaning: "No longer recommended",
    tone: "neutral",
  },
  "decision.superseded": {
    label: "Superseded",
    meaning: "Replaced by a newer NNNN",
    tone: "neutral",
  },
  "architecture.draft": {
    label: "Draft",
    meaning: "Being written",
    tone: "warn",
  },
  "architecture.current": {
    label: "Current",
    meaning: "Describes the live system",
    tone: "ok",
  },
  "architecture.superseded": {
    label: "Superseded",
    meaning: "No longer the current shape",
    tone: "neutral",
  },
  "proposal.draft": { label: "Draft", meaning: "Being written", tone: "warn" },
  "proposal.review": {
    label: "Review",
    meaning: "Awaiting/under acceptance review",
    tone: "warn",
  },
  "proposal.accepted": { label: "Accepted", meaning: "Approved as the choice", tone: "ok" },
  "proposal.rejected": { label: "Rejected", meaning: "Declined", tone: "danger" },
  "proposal.superseded": { label: "Superseded", meaning: "Replaced", tone: "neutral" },
  "prompt.draft": { label: "Draft", meaning: "Being written", tone: "warn" },
  "prompt.active": { label: "Active", meaning: "Reusable/current", tone: "ok" },
  "prompt.archived": { label: "Archived", meaning: "Retired", tone: "neutral" },
  "research.draft": { label: "Draft", meaning: "Being written", tone: "warn" },
  "research.active": { label: "Active", meaning: "Current", tone: "ok" },
  "research.archived": { label: "Archived", meaning: "Retired", tone: "neutral" },
  "questions.active": {
    label: "Active",
    meaning: "Awaiting resolution",
    tone: "danger",
  },
  "questions.resolved": {
    label: "Resolved",
    meaning: "Answered, kept as a record",
    tone: "ok",
  },
  "wiki.draft": { label: "Draft", meaning: "Being written", tone: "warn" },
  "wiki.active": { label: "Active", meaning: "Live page", tone: "ok" },
  "wiki.archived": { label: "Archived", meaning: "Retired", tone: "neutral" },
  "commands.active": {
    label: "Active",
    meaning: "Thin per-trigger stub, in place",
    tone: "ok",
  },
  "archive.archived": {
    label: "Archived",
    meaning: "Historical record under archive/",
    tone: "neutral",
  },
};

/** Lookup a closed-vocabulary value for a kind; unknown values return null. */
export function valueLabel(kind: string, value: string): ValueLabel | null {
  const v = value.trim().toLowerCase();
  return STATUS_VALUES[`${kind}.${v}`] ?? null;
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
