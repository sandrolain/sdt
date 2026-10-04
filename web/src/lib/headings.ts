/**
 * Remove a single leading top-level heading from a markdown body. The viewer
 * renders the document title in its header, so a body `# Title` would be a
 * duplicate; this must be applied wherever the body is interpreted (render and
 * outline) so heading indices stay aligned.
 */
export function stripLeadingH1(markdown: string): string {
  return markdown.replace(/^\s*#\s+.*(?:\r?\n|$)/, "");
}

/**
 * Normalise heading text for display and comparison: strip inline markdown
 * syntax (backtick code spans, `*`/`_` emphasis, `[text](target)` links and
 * images), collapse whitespace and trim. `parseOutline` keeps the raw markdown
 * while the renderer's `data-heading` is the inline-parsed plain text, so both
 * sides are normalised before the Sections comparison, the jump target and the
 * published active heading, and the TOC text matches the rendered heading.
 */
export function normalizeHeadingText(text: string): string {
  return text
    .replace(/`([^`]*)`/g, "$1")
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/(\*\*|__)(.*?)\1/g, "$2")
    .replace(/(\*|_)(.*?)\1/g, "$2")
    .replace(/\s+/g, " ")
    .trim();
}
