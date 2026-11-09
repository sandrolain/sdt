/**
 * Normalize the markdown handed to Marp.
 *
 * Marp reads its frontmatter (the `theme:`, `paginate:`, `size:` … block) only
 * when it is the first thing in the document. A material deck puts its machine
 * header comment first (`<!-- material: … -->`, decision 0038) and the deck
 * frontmatter after it; Marp then reads the header as slide 1 and the
 * frontmatter keys as slide 2 content, so the deck opens on a blank slide and
 * shows `theme: default` / `paginate: true` as text.
 *
 * A leading HTML comment followed by a frontmatter block is that shape: the
 * comment is a file header, not slide content, so it is dropped and the
 * frontmatter moves to the start. A leading comment followed by a separator is
 * left alone — it is a speaker note on slide 1, ordinary Marp.
 */

/** A leading HTML comment immediately followed by a `---` frontmatter block. */
const LEADING_COMMENT_BEFORE_FRONTMATTER = /^\s*<!--[\s\S]*?-->\s*\n+(?=---\n[^\n])/;

/** Move a leading file-header comment out of the way of Marp's frontmatter. */
export function normalizeDeckSource(source: string): string {
  return source.replace(LEADING_COMMENT_BEFORE_FRONTMATTER, "");
}
