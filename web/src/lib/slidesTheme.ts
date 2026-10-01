/**
 * The slide theme, authored from the viewer's design tokens.
 *
 * Marp themes use a `@theme` header and `section` selectors. This theme
 * consumes the semantic aliases in `web/src/styles/tokens.css` (`--bg-base`,
 * `--fg`, `--accent`, …) rather than Marp's default look or any raw colour, so
 * the deck follows the active light/dark theme and the elevation roles without a
 * second palette. The rules append after Marp's structural CSS and KaTeX's
 * stylesheet, so they win where they overlap.
 */

/** Theme name registered on the engine's themeSet and named by a deck. */
export const SLIDES_THEME_NAME = "sdt";

/**
 * The Marp theme CSS. `section` is the slide element; `:where()` keeps the
 * specificity low so a deck's `class` spot directive can still override.
 */
export const SLIDES_THEME_CSS = `/* @theme ${SLIDES_THEME_NAME} */
section {
  background: var(--bg-base);
  color: var(--fg);
  font-family: var(--font-sans, system-ui, sans-serif);
  padding: 60px 72px;
  font-size: 26px;
  line-height: 1.4;
}
h1, h2, h3, h4, h5, h6 {
  color: var(--fg);
  line-height: 1.2;
}
h1 { font-size: 48px; border-bottom: 3px solid var(--accent); padding-bottom: 12px; }
h2 { font-size: 38px; }
a { color: var(--accent); }
strong { color: var(--fg); }
code {
  background: var(--bg-crust);
  border-radius: 4px;
  padding: 0.1em 0.35em;
}
pre {
  background: var(--bg-mantle);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px 20px;
}
pre code { background: none; padding: 0; }
blockquote {
  border-left: 4px solid var(--accent);
  margin-left: 0;
  padding-left: 20px;
  color: var(--fg-dim);
}
table { border-collapse: collapse; }
th, td { border: 1px solid var(--border); padding: 6px 12px; }
th { background: var(--bg-mantle); }
li::marker { color: var(--accent); }
section::after {
  color: var(--fg-faint);
}
section.lead { text-align: center; }
section.lead h1 { border-bottom: none; }
`;

/** The full stylesheet a slide frame injects: the token theme only. */
export function slidesThemeCss(): string {
  return SLIDES_THEME_CSS;
}
