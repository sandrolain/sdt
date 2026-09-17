import type { Tokens, TokenizerAndRendererExtension } from "marked";

interface DefItem {
  termTokens: unknown;
  defs: unknown[];
}

interface DeflistToken extends Tokens.Generic {
  type: "deflist";
  items: DefItem[];
}

interface DeflistThis {
  parser: { parseInline(tokens: unknown): string };
}

/**
 * Minimal definition-list extension:
 *
 * ```
 * term
 * : definition
 * ```
 *
 * Consecutive `: definition` lines belong to the same term; multiple
 * term/definition groups form one `<dl>`. Inline tokens are produced by the
 * tokenizer (where the lexer is available) and parsed in the renderer.
 */
const deflist: TokenizerAndRendererExtension = {
  name: "deflist",
  level: "block",
  start(src) {
    return /\S[^\n]*\n:[ \t]/.exec(src)?.index;
  },
  tokenizer(src) {
    const lines = src.split("\n");
    const items: DefItem[] = [];
    let i = 0;
    while (i < lines.length) {
      const term = lines[i];
      if (term.trim() === "" || term.startsWith(":") || !lines[i + 1]?.startsWith(":")) break;
      const termTokens = this.lexer.inlineTokens(term.trim());
      i += 1;
      const defs: unknown[] = [];
      while (i < lines.length && lines[i].startsWith(":")) {
        defs.push(this.lexer.inlineTokens(lines[i].replace(/^:[ \t]?/, "")));
        i += 1;
      }
      items.push({ termTokens, defs });
    }
    if (items.length === 0) return undefined;
    return { type: "deflist", raw: lines.slice(0, i).join("\n"), items } as DeflistToken;
  },
  renderer(token) {
    const { items } = token as DeflistToken;
    const host = this as unknown as DeflistThis;
    const body = items
      .map((item) => {
        const defs = item.defs.map((def) => `<dd>${host.parser.parseInline(def)}</dd>`).join("");
        return `<dt>${host.parser.parseInline(item.termTokens)}</dt>${defs}`;
      })
      .join("");
    return `<dl class="md-deflist">${body}</dl>`;
  },
};

/** Definition-list marked extension. */
export function deflistExtension(): TokenizerAndRendererExtension[] {
  return [deflist];
}
