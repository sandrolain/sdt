/**
 * The slide lane's markdown-it fence plugin.
 *
 * Marp compiles a fenced block to a highlighted `<pre><code>`; the viewer's
 * diagram renderer (Pele) expects the `.md-mermaid[data-src]` placeholder the
 * ordinary markdown renderer emits. This plugin is the one seam that decides a
 * fence is Mermaid: it returns the placeholder for `mermaid`, delegates every
 * other language to the rule Marp Core installed, and escapes the attribute.
 *
 * It is installed through `marp.use()` (Marpit's markdown-it/PostCSS plugin
 * seam), after construction, so it wraps the final fence rule.
 */

interface FenceToken {
  info: string;
  content: string;
}

type FenceRule = (
  tokens: FenceToken[],
  idx: number,
  options: unknown,
  env: unknown,
  self: unknown,
) => string;

interface MarkdownItFence {
  renderer: { rules: { fence?: FenceRule } };
}

/** Escape a value for a double-quoted HTML attribute. */
function escapeAttribute(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

/** The first word of a fence info string (`mermaid` or `mermaid title`). */
function fenceLanguage(info: string): string {
  return info.trim().split(/\s+/)[0];
}

/** Emit `.md-mermaid[data-src]` for a mermaid fence; delegate all others. */
export function mermaidFencePlugin(md: MarkdownItFence): void {
  const fence = md.renderer.rules.fence;
  if (!fence) return;
  md.renderer.rules.fence = (tokens, idx, options, env, self) => {
    const token = tokens[idx];
    if (fenceLanguage(token.info) === "mermaid") {
      const source = escapeAttribute(encodeURIComponent(token.content));
      return `<div class="md-mermaid" data-src="${source}"></div>`;
    }
    return fence(tokens, idx, options, env, self);
  };
}
