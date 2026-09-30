/** Newline-separated line numbers matching a source's line count. */
export function lineNumbers(source: string): string {
  const count = source.split("\n").length;
  return Array.from({ length: count }, (_, i) => i + 1).join("\n");
}

/** Parse a fenced-code info string into a language and highlighted line set. */
export interface CodeInfo {
  language: string;
  highlighted: Set<number>;
}

/** `js {1,3-5}` → language `js`, lines {1,3,4,5}. */
export function parseCodeInfo(info?: string): CodeInfo {
  const match = /^([^\s{]+)?\s*\{([^}]*)\}/.exec((info ?? "").trim());
  const language = match?.[1] ?? (info ?? "").trim().split(/\s+/)[0] ?? "";
  const highlighted = new Set<number>();
  for (const part of (match?.[2] ?? "").split(",")) {
    const range = /^(\d+)\s*-\s*(\d+)$/.exec(part.trim());
    if (range) {
      const [from, to] = [Number(range[1]), Number(range[2])];
      for (let i = Math.min(from, to); i <= Math.max(from, to); i++) highlighted.add(i);
    } else if (/^\d+$/.test(part.trim())) {
      highlighted.add(Number(part.trim()));
    }
  }
  return { language, highlighted };
}

/**
 * Split highlighted HTML into per-line fragments with balanced spans: any span
 * left open at a line break is closed at the end of the line and reopened at
 * the start of the next, so each fragment can be wrapped without breaking
 * multi-line tokens (block comments, template strings).
 */
export function splitHighlightedLines(html: string): string[] {
  const stack: string[] = [];
  return html.split("\n").map((line) => {
    const carry = stack.join("");
    const tags = line.match(/<span\b[^>]*>|<\/span>/g);
    if (tags) {
      for (const tag of tags) {
        if (tag === "</span>") stack.pop();
        else stack.push(tag);
      }
    }
    return `${carry}${line}${"</span>".repeat(stack.length)}`;
  });
}

/**
 * Wrap highlighted HTML lines in `.md-code__line`, marking the highlighted set.
 * The wrappers are block-level and joined without a newline: inside `<pre>`
 * (`white-space: pre`) an inter-span newline would render as a blank line
 * between every source line. Copy re-joins them with `\n` via `codeBlockText`.
 */
export function wrapHighlightedLines(html: string, highlighted: Set<number>): string {
  return splitHighlightedLines(html)
    .map(
      (line, index) =>
        `<span class="md-code__line${highlighted.has(index + 1) ? " is-hl" : ""}">${line}</span>`,
    )
    .join("");
}

/**
 * Plain text of a rendered `<code>`, with `.md-code__line` wrappers re-joined by
 * newlines (they carry none of their own). Elements without the wrappers fall
 * back to their `textContent`.
 */
export function codeBlockText(code: Element | null | undefined): string {
  if (!code) return "";
  const lines = code.querySelectorAll(".md-code__line");
  if (lines.length === 0) return code.textContent ?? "";
  return Array.from(lines, (line) => line.textContent ?? "").join("\n");
}
