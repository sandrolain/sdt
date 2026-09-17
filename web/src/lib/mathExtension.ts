import type { TokenizerAndRendererExtension, Tokens } from "marked";

/** Escape a value for an HTML attribute. */
function escapeAttr(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

interface MathToken extends Tokens.Generic {
  type: "mathBlock" | "mathInline";
  text: string;
  display: boolean;
}

const blockMath: TokenizerAndRendererExtension = {
  name: "mathBlock",
  level: "block",
  start(src) {
    return src.match(/\$\$|\\\[/)?.index;
  },
  tokenizer(src): MathToken | undefined {
    const match = /^\$\$([\s\S]+?)\$\$(?:\n|$)/.exec(src) ?? /^\\\[([\s\S]+?)\\\]/.exec(src);
    if (!match) return undefined;
    return {
      type: "mathBlock",
      raw: match[0],
      text: match[1].trim(),
      display: true,
    } as MathToken;
  },
  renderer(token) {
    const math = token as MathToken;
    return `<span class="md-math md-math--block" data-display="block" data-tex="${escapeAttr(math.text)}"></span>`;
  },
};

const inlineMath: TokenizerAndRendererExtension = {
  name: "mathInline",
  level: "inline",
  start(src) {
    return src.match(/\$|\\\(/)?.index;
  },
  tokenizer(src): MathToken | undefined {
    // `$$…$$` is handled by the block tokenizer; only single `$` here
    const match = /^\$([^$\n]+?)\$/.exec(src) ?? /^\\\(([\s\S]+?)\\\)/.exec(src);
    if (!match) return undefined;
    return {
      type: "mathInline",
      raw: match[0],
      text: match[1].trim(),
      display: false,
    } as MathToken;
  },
  renderer(token) {
    const math = token as MathToken;
    return `<span class="md-math" data-display="inline" data-tex="${escapeAttr(math.text)}"></span>`;
  },
};

/** marked extensions rendering `$…$`/`$$…$$`/`\(…\)`/`\[…\]` as KaTeX targets. */
export function mathExtensions(): TokenizerAndRendererExtension[] {
  return [blockMath, inlineMath];
}
