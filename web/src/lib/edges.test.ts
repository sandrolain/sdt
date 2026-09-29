// @vitest-environment node
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { plansByAnalysis, tasksByPlan } from "./statusDot";
import type { TreeEntry } from "./api";

/**
 * The viewer half of the shared lifecycle-edge contract: the same
 * `expected-edges.json` the Go resolver is asserted against
 * (cli/cmd/context_edges_test.go) is read here, over the same fixture corpus.
 * The SPA does not resolve anything itself — the server stamps the edges on the
 * payload — so this test builds the payload the server would serve from the
 * fixture's typed relations and checks the two indexes against the contract.
 */
const fixtureDir = fileURLToPath(
  new URL("../../../cli/cmd/testdata/lifecycle-edges", import.meta.url),
);

interface EdgeExpectation {
  parent: string;
  children: string[];
}

function loadFixture(): Record<string, EdgeExpectation> {
  const raw = readFileSync(`${fixtureDir}/expected-edges.json`, "utf8");
  return (JSON.parse(raw) as { edges: Record<string, EdgeExpectation> }).edges;
}

function readDoc(rel: string): string {
  return readFileSync(`${fixtureDir}/${rel}`, "utf8");
}

function field(content: string, key: string): string {
  return new RegExp(`^${key}:\\s*(.*)$`, "m").exec(content)?.[1]?.trim() ?? "";
}

function listField(content: string, key: string): string[] {
  const block = new RegExp(`^${key}:\\s*\\n((?:[ \\t]*-[^\\n]*\\n?)+)`, "m").exec(content);
  if (!block) return [];
  return block[1]
    .split("\n")
    .filter((line) => line.trim().startsWith("-"))
    .map((line) =>
      line
        .trim()
        .replace(/^-\s*/, "")
        .replace(/^["']|["']$/g, ""),
    );
}

/** The tree payload the server builds from the fixture corpus: the resolved
 *  `analysis`/`plans`/`plan` fields, plus the `sources` prose it still serves.
 *  Entries come in path order, exactly as `walkTree` sorts them, so the index
 *  buckets are deterministic. */
function payload(): TreeEntry[] {
  const edges = loadFixture();
  const entries: TreeEntry[] = [];
  for (const [rel, want] of Object.entries(edges)) {
    const content = readDoc(rel);
    const kind = field(content, "kind");
    const entry: TreeEntry = { path: rel, kind, sources: listField(content, "sources") };
    if (kind === "plan") entry.analysis = want.parent;
    if (kind === "analysis") entry.plans = want.children;
    if (kind === "tasks") entry.plan = want.parent;
    entries.push(entry);
  }
  return entries.sort((a, b) => a.path.localeCompare(b.path));
}

describe("the shared lifecycle-edge fixture", () => {
  it("is a corpus the fixture describes (a guard on the fixture itself)", () => {
    for (const dir of ["analysis", "plan", "tasks"]) {
      expect(readdirSync(`${fixtureDir}/context/${dir}`).length).toBeGreaterThan(0);
    }
    expect(Object.keys(loadFixture()).length).toBeGreaterThanOrEqual(9);
  });

  it("gives the payload the edges the contract names", () => {
    const edges = loadFixture();
    for (const [ref, want] of Object.entries(edges)) {
      const entry = payload().find((e) => e.path === ref);
      expect(entry, ref).toBeTruthy();
      if (entry?.kind === "plan") expect(entry.analysis, ref).toBe(want.parent);
      if (entry?.kind === "analysis") expect(entry.plans ?? [], ref).toEqual(want.children);
      if (entry?.kind === "tasks") expect(entry.plan ?? "", ref).toBe(want.parent);
    }
  });

  it("indexes the fixture corpus the way the contract describes", () => {
    const edges = loadFixture();
    const entries = payload();
    const analyses = plansByAnalysis(entries);
    const tasks = tasksByPlan(entries);
    for (const [ref, want] of Object.entries(edges)) {
      if (want.children.length > 0) {
        // an analysis indexes the plans naming it; a plan indexes its task files
        const child = ref.startsWith("context/analysis/") ? analyses.get(ref) : tasks.get(ref);
        expect(
          (child ?? []).map((e) => e.path),
          ref,
        ).toEqual(want.children);
      }
      // every task file lands under exactly the plan the contract names
      if (ref.startsWith("context/tasks/")) {
        const bucket = want.parent ? (tasks.get(want.parent) ?? []) : (tasks.get("") ?? []);
        expect(
          bucket.map((e) => e.path),
          ref,
        ).toContain(ref);
      }
    }
    // the cited sibling never appears as a parent of anything
    expect(analyses.has("context/analysis/sibling.md")).toBe(false);
    // the links-only task file stays in the no-plan bucket
    expect((tasks.get("") ?? []).map((e) => e.path)).toContain("context/tasks/linked.md");
  });

  it("does not read `sources` for an edge: re-wiring the payload cannot change the indexes", () => {
    const entries = payload();
    // a payload whose `sources` is emptied keeps every edge
    const stripped = entries.map((e) => ({ ...e, sources: [] }));
    expect([...plansByAnalysis(stripped).keys()].sort()).toEqual(
      [...plansByAnalysis(entries).keys()].sort(),
    );
    expect([...tasksByPlan(stripped).keys()].sort()).toEqual(
      [...tasksByPlan(entries).keys()].sort(),
    );
  });
});
