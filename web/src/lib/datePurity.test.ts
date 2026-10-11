/// <reference types="node" />
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Date purity in components.
 *
 * Reading the current time inside a render body makes a render
 * non-deterministic: it cannot be memoised, it differs between two renders of
 * the same props, and a test must stub the clock to be stable. The failure is
 * easy to reintroduce, so the rule is a check, not prose.
 *
 * The rule flags only a **no-argument** time read — `Date.now()`,
 * `new Date()`, `performance.now()` — because those read "now". A `new Date(x)`
 * with an argument is a deterministic conversion and is not flagged. A
 * module-scope declaration (column 0) is hoisted out of render and is the
 * intended escape hatch: `const NOW = Date.now()` runs once at import.
 *
 * Scope: `.tsx` files (component and JSX-bearing modules). `lib/*.ts` time
 * logic (event buffers, scroll timestamps, calendar math) is deliberately out
 * of scope — it is not a render path.
 */

const srcDir = new URL("..", import.meta.url).pathname;
const webDir = join(srcDir, "..");

const NOW_READ =
  /\b(?:Date\s*\.\s*now\s*\(\s*\)|new\s+Date\s*\(\s*\)|performance\s*\.\s*now\s*\(\s*\))/;

/** stripLiterals blanks quoted strings and line comments, so a URL or a message
 * that merely spells a call does not read as one. It is deliberately a small
 * best-effort normaliser: this is a source scan, not a parser. */
function stripLiterals(line: string): string {
  return line
    .replace(/"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'/g, " ")
    .replace(/\/\/.*$/, "")
    .replace(/\/\*[\s\S]*?\*\//g, " ");
}

/** nowReads returns the 1-based line numbers of no-argument time reads in
 * `source` that are not module-scope declarations.
 *
 * Known limits of the line-based heuristic — a now-read in one of these shapes
 * is reported, missed or missed respectively, and the check accepts all three:
 * a call split across lines; a call inside a multi-line block comment; a
 * one-line top-level arrow function (`const C = () => Date.now()`), which is
 * indistinguishable from the module-scope escape hatch. */
export function nowReads(source: string): number[] {
  const out: number[] = [];
  source.split("\n").forEach((line, i) => {
    const code = stripLiterals(line);
    if (!NOW_READ.test(code)) return;
    if (!/^\s/.test(code)) return; // module scope: hoisted out of render
    out.push(i + 1);
  });
  return out;
}

function tsxFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...tsxFiles(path));
    } else if (/\.tsx$/.test(entry.name) && !/\.(test|spec)\./.test(entry.name)) {
      out.push(path);
    }
  }
  return out;
}

describe("date purity in components", () => {
  it("flags a now-read inside a function body", () => {
    const source = ["function Clock() {", "  const now = Date.now();", "  return now;", "}"].join(
      "\n",
    );
    expect(nowReads(source)).toEqual([2]);
  });

  it("flags no-argument new Date() and performance.now() in a body", () => {
    const source = [
      "export function Stamp() {",
      "  const d = new Date();",
      "  const t = performance.now();",
      "  return `${d}${t}`;",
      "}",
    ].join("\n");
    expect(nowReads(source)).toEqual([2, 3]);
  });

  it("passes a module-scope declaration", () => {
    expect(nowReads("const NOW = Date.now();")).toEqual([]);
    expect(nowReads("const TODAY = new Date().toISOString().slice(0, 10);")).toEqual([]);
  });

  it("passes a deterministic conversion with an argument", () => {
    const source = [
      "function Day({ y, m }: { y: number; m: number }) {",
      "  return new Date(Date.UTC(y, m, 1));",
      "}",
    ].join("\n");
    expect(nowReads(source)).toEqual([]);
  });

  it("sees through whitespace in the call", () => {
    expect(nowReads("function C() {\n  const n = Date.now( );\n  return n;\n}")).toEqual([2]);
    expect(nowReads("function C() {\n  return new Date ();\n}")).toEqual([2]);
  });

  it("does not read a call spelled inside a string", () => {
    expect(nowReads('function C() {\n  return "call Date.now() to read time";\n}')).toEqual([]);
    expect(
      nowReads("function C() {\n  const u = 'http://x'; const n = 1;\n  return u + n;\n}"),
    ).toEqual([]);
  });

  it("finds no now-read in a component body across the app sources", () => {
    const files = tsxFiles(srcDir);
    // Coverage guard: a walk that finds nothing would pass vacuously.
    expect(files.length).toBeGreaterThan(0);
    const offenders: string[] = [];
    for (const file of files) {
      for (const line of nowReads(readFileSync(file, "utf8"))) {
        offenders.push(`${file.replace(webDir, "web")}:${line}`);
      }
    }
    expect(offenders).toEqual([]);
  });
});
