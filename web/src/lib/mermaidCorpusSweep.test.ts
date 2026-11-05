// @vitest-environment node
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { render } from "pele";

/**
 * The corpus contract for the diagram lane, over the whole corpus: every fenced
 * `mermaid` block the viewer can serve must parse. `mermaidCorpus.test.ts` pins
 * the three kinds the corpus uses and the theme binding; this sweep is the set,
 * so a diagram added later cannot reach a reader as the source fallback in a red
 * box without a failing test naming its file and line.
 *
 * Scope mirrors the served corpus (`viewer/server.go`, `handleTree`): `refs/` is
 * an immutable clone, `tmp/` and `scripts/` hold no documents.
 */
const CORPUS_ROOT = fileURLToPath(new URL("../../../context", import.meta.url));

/** Directories the viewer does not serve. */
const UNSERVED = new Set(["refs", "tmp", "scripts"]);

function corpusMarkdown(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.name.startsWith(".")) continue;
    const full = join(dir, entry.name);
    if (entry.isDirectory()) {
      if (!UNSERVED.has(entry.name)) corpusMarkdown(full, out);
    } else if (entry.name.endsWith(".md")) {
      out.push(full);
    }
  }
  return out;
}

/** Every fenced `mermaid` block of a document, with the line that opens it. */
function mermaidBlocks(text: string): { line: number; source: string }[] {
  const lines = text.split("\n");
  const blocks: { line: number; source: string }[] = [];
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].trim() !== "```mermaid") continue;
    let end = i + 1;
    while (end < lines.length && lines[end].trim() !== "```") end++;
    blocks.push({ line: i + 1, source: lines.slice(i + 1, end).join("\n") });
    i = end;
  }
  return blocks;
}

describe("mermaid corpus sweep", () => {
  // `context/` is gitignored: the corpus belongs to the machine that bootstrapped
  // it, so a checkout without one skips the sweep instead of failing it.
  it.skipIf(!existsSync(CORPUS_ROOT))(
    "parses every fenced mermaid block in the served corpus",
    () => {
      const files = corpusMarkdown(CORPUS_ROOT);
      expect(files.length).toBeGreaterThan(0);

      const unparseable: string[] = [];
      let blocks = 0;
      for (const file of files) {
        for (const block of mermaidBlocks(readFileSync(file, "utf8"))) {
          blocks++;
          try {
            render(block.source);
          } catch (error) {
            const reason = error instanceof Error ? error.message.split("\n")[0] : String(error);
            unparseable.push(`${relative(CORPUS_ROOT, file)}:${block.line} — ${reason}`);
          }
        }
      }

      expect(blocks).toBeGreaterThan(0);
      expect(unparseable).toEqual([]);
    },
  );
});
