/**
 * Label-layer CSS for the ported graph engine.
 *
 * Ported faithfully from `context/refs/react/graph-react/src/KnowledgeGraph.jsx`
 * (LABEL_CSS/EDGE_LABEL_CSS 224-233). The DOM label layer is the medium of the
 * engine, so these raw values are graph identity (phase 9), not theme surfaces;
 * the Catppuccin theming phase may re-pick them but must keep them raw here.
 */

export const LABEL_CSS =
  "position:absolute;left:0;top:0;white-space:pre-line;pointer-events:none;" +
  "font:400 11.5px/16px system-ui,-apple-system,'Segoe UI',sans-serif;" +
  "color:rgba(226,232,240,.92);text-shadow:0 0 5px #05080f,0 0 2px #05080f,0 1px 2px #05080f;" +
  "will-change:transform;display:none;" +
  // A label is bounded: the placement pass wraps it, and these two declarations are
  // the visible half of the same bound (analysis 20261013-063009, D3). The max-width
  // is a safety net for a mis-estimated line; overflow stays hidden rather than
  // spilling over the map.
  "max-width:180px;overflow-wrap:anywhere;overflow:hidden;";

/**
 * The label box: the text wrapped into at most `LABEL_MAX_LINES` lines, with the
 * width and height the placement pass reserves. One function answers both halves of
 * the label bound, so the rendered text and the collision box cannot disagree —
 * which is what an unbounded `nowrap` label got wrong in both directions: a long
 * title either dominated the canvas or was dropped as a collision forever.
 */
export const LABEL_MAX_WIDTH = 180;
export const LABEL_MAX_LINES = 2;
export const LABEL_LINE_HEIGHT = 16;
/** Estimated advance of one character at 11.5px system-ui: the ported engine's own
 * estimate, kept beside the CSS so the wrap and the box measure the same text. */
export const LABEL_CHAR_WIDTH = 6.3;
/** Horizontal padding the ported engine added to its estimate (10px). */
export const LABEL_PAD = 10;

export interface LabelBox {
  /** The lines to render, at most `LABEL_MAX_LINES`; the last carries an ellipsis
   * when the title did not fit. */
  lines: string[];
  width: number;
  height: number;
  truncated: boolean;
}

/**
 * Wrap `text` into at most two lines of `LABEL_MAX_WIDTH` and report the box. Text
 * is wrapped on words; a word longer than a line is hard-split so a single long
 * path cannot overflow the bound. Empty text yields one empty line, which keeps the
 * caller's arithmetic total.
 */
export function labelBox(text: string): LabelBox {
  const maxChars = Math.max(4, Math.floor((LABEL_MAX_WIDTH - LABEL_PAD) / LABEL_CHAR_WIDTH));
  const words: string[] = [];
  for (const word of text.trim().split(/\s+/)) {
    if (!word) continue;
    if (word.length <= maxChars) {
      words.push(word);
      continue;
    }
    for (let i = 0; i < word.length; i += maxChars) words.push(word.slice(i, i + maxChars));
  }

  const lines: string[] = [];
  let truncated = false;
  for (const word of words) {
    const last = lines[lines.length - 1];
    if (last !== undefined && last.length + 1 + word.length <= maxChars) {
      lines[lines.length - 1] = `${last} ${word}`;
      continue;
    }
    if (lines.length === LABEL_MAX_LINES) {
      truncated = true;
      break;
    }
    lines.push(word);
  }
  if (lines.length === 0) lines.push("");
  if (truncated) {
    const index = lines.length - 1;
    const last = lines[index];
    lines[index] = last.length >= maxChars ? `${last.slice(0, maxChars - 1)}…` : `${last}…`;
  }

  const longest = lines.reduce((max, line) => Math.max(max, line.length), 0);
  return {
    lines,
    width: Math.min(LABEL_MAX_WIDTH, longest * LABEL_CHAR_WIDTH + LABEL_PAD),
    height: lines.length * LABEL_LINE_HEIGHT,
    truncated,
  };
}

export const EDGE_LABEL_CSS =
  "position:absolute;left:0;top:0;white-space:nowrap;pointer-events:none;display:none;" +
  "font:500 10.5px/14px system-ui,-apple-system,'Segoe UI',sans-serif;" +
  "padding:1px 6px;border-radius:8px;background:rgba(8,12,22,.82);" +
  "border:1px solid rgba(148,163,184,.25);will-change:transform;";
