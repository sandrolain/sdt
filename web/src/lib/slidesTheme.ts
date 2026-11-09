/**
 * The slide theme, authored from the viewer's design tokens.
 *
 * Marp themes use a `@theme` header and `section` selectors. This theme
 * consumes the semantic aliases in `web/src/styles/tokens.css` (`--bg-base`,
 * `--fg`, `--accent`, …) rather than Marp's default look or any raw colour, so
 * the deck follows the active light/dark theme and the elevation roles without a
 * second palette. The rules append after Marp's structural CSS and KaTeX's
 * stylesheet, so they win where they overlap.
 *
 * The same body is registered twice: `sdt` is the named theme a deck may ask
 * for, and `default` replaces Marp's built-in theme, because the packed
 * stylesheet of a deck that names no theme (or says `theme: default`, as the
 * corpus decks do) would otherwise be the 96 KB GitHub look. A deck that names
 * another official theme (`gaia`, `uncover`) still gets that theme: it is an
 * explicit deck choice, not the default.
 */

/** Theme name registered on the engine's themeSet and named by a deck. */
export const SLIDES_THEME_NAME = "sdt";

/** The built-in name overridden with the same body, so the default look is ours. */
export const SLIDES_DEFAULT_THEME_NAME = "default";

/**
 * The theme rules. `section` is the slide element; `:where()` keeps the
 * specificity low so a deck's `class` spot directive can still override.
 */
const SLIDES_THEME_BODY = `section {
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
h3 { font-size: 30px; }
p, ul, ol, blockquote, pre, table { margin: 0.55em 0; }
ul, ol { padding-left: 1.2em; }
li { margin: 0.18em 0; }
a { color: var(--accent); }
strong { color: var(--fg); }
em { color: var(--fg); }
hr { border: none; border-top: 2px solid var(--border); margin: 1em 0; }
img { max-width: 100%; height: auto; }
code {
  background: var(--bg-crust);
  border-radius: 4px;
  padding: 0.1em 0.35em;
  font-family: ui-monospace, "SF Mono", monospace;
  font-size: 0.9em;
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
table { border-collapse: collapse; max-width: 100%; }
th, td { border: 1px solid var(--border); padding: 6px 12px; }
th { background: var(--bg-mantle); }
li::marker { color: var(--accent); }
header, footer { color: var(--fg-faint); }
section::after {
  color: var(--fg-faint);
}
section.lead { text-align: center; }
section.lead h1 { border-bottom: none; }
`;

/** The named `sdt` theme CSS. */
export const SLIDES_THEME_CSS = `/* @theme ${SLIDES_THEME_NAME} */\n${SLIDES_THEME_BODY}`;

/** The same body under Marp's built-in name, so an unset theme is still ours. */
export const SLIDES_DEFAULT_THEME_CSS = `/* @theme ${SLIDES_DEFAULT_THEME_NAME} */\n${SLIDES_THEME_BODY}`;

/** The stylesheet a slide frame registers under the `sdt` name. */
export function slidesThemeCss(): string {
  return SLIDES_THEME_CSS;
}

/** The stylesheet that replaces Marp's built-in default theme. */
export function slidesDefaultThemeCss(): string {
  return SLIDES_DEFAULT_THEME_CSS;
}
