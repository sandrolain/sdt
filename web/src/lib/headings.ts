/**
 * Remove a single leading top-level heading from a markdown body. The viewer
 * renders the document title in its header, so a body `# Title` would be a
 * duplicate; this must be applied wherever the body is interpreted (render and
 * outline) so heading indices stay aligned.
 */
export function stripLeadingH1(markdown: string): string {
  return markdown.replace(/^\s*#\s+.*(?:\r?\n|$)/, "");
}
