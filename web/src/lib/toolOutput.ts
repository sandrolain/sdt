/**
 * How a command's output is read in the agent panel.
 *
 * The panel gets a command's stdout as one string and has no language on it. Guessing with
 * highlight.js's `highlightAuto` is unbounded work on a 400-line log and guesses worse than
 * the output itself does, so the rule is deliberate and small: JSON when it parses, a diff
 * when it looks like one, plaintext otherwise — and plaintext above the cap, where a
 * highlight costs more than it tells the reader.
 */
import { highlightCode } from "./markdown";

/** Above this, an output stays plaintext: a highlight would cost more than it tells. */
export const HIGHLIGHT_MAX_CHARS = 20_000;
/** The same ceiling in lines, for an output of many short lines. */
export const HIGHLIGHT_MAX_LINES = 400;

export type ToolOutputLanguage = "json" | "diff" | "plaintext";

/** toolOutputLanguage is the language an output names by its own shape. */
export function toolOutputLanguage(text: string): ToolOutputLanguage {
  const trimmed = text.trimStart();
  if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
    try {
      JSON.parse(text);
      return "json";
    } catch {
      // a brace is not a JSON document: fall through to the other shapes
    }
  }
  if (/^diff --git /m.test(text) || /^@@ /m.test(text)) return "diff";
  return "plaintext";
}

/** highlightedToolOutput is the output as hljs HTML, with the language it was read as. */
export function highlightedToolOutput(text: string): {
  html: string;
  language: ToolOutputLanguage;
} {
  const overCap =
    text.length > HIGHLIGHT_MAX_CHARS || text.split("\n").length > HIGHLIGHT_MAX_LINES;
  const language: ToolOutputLanguage = overCap ? "plaintext" : toolOutputLanguage(text);
  return { html: highlightCode(text, language), language };
}
