/// <reference types="node" />
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Guard for D7: the Vite template residue (`web/src/index.css`) and the unused
 * template assets shipped for months without a single import. Deleting dead
 * weight is only durable if nothing can quietly reintroduce it.
 */

const srcDir = new URL("..", import.meta.url).pathname;
const webDir = join(srcDir, "..");

function sourceFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...sourceFiles(path));
    } else if (/\.(ts|tsx|css)$/.test(entry.name) && !/\.(test|spec)\./.test(entry.name)) {
      out.push(path);
    }
  }
  return out;
}

describe("dead source assets", () => {
  it("keeps the Vite template stylesheet deleted", () => {
    expect(existsSync(join(srcDir, "index.css"))).toBe(false);
  });

  it("keeps the unused template assets deleted", () => {
    expect(existsSync(join(srcDir, "assets"))).toBe(false);
  });

  it("references no dead asset from the app sources", () => {
    const offenders: string[] = [];
    for (const file of [...sourceFiles(srcDir), join(webDir, "index.html")]) {
      const text = readFileSync(file, "utf8");
      if (/(?:^|["'(])\.\/index\.css/.test(text) || /assets\//.test(text)) {
        offenders.push(file.replace(webDir, "web"));
      }
    }
    expect(offenders).toEqual([]);
  });
});
