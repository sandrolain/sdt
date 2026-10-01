import DOMPurify, { type Config } from "dompurify";

/**
 * The slide-scoped sanitizer.
 *
 * The global markdown allowlist in `markdown.ts` deliberately excludes `style`
 * and `svg`. Rendered Marp slides need a wider set: KaTeX emits MathML plus
 * inline SVG even with `inlineSVG: false` (decision 0022), and Marp puts layout
 * directives on the `section` as `style`/`data-*` attributes. This config
 * allows exactly that rendered surface while still stripping script and event
 * handlers; the global allowlist is never touched.
 *
 * `style` is allowed as an **attribute** (Marp's per-slide CSS variables and
 * KaTeX's inline sizing) but not as a `<style>` tag from slide bodies — the
 * theme stylesheet is a component-owned element, injected outside this
 * sanitizer.
 */

/** Tags a rendered slide can contain: markdown, plus the KaTeX MathML/SVG set. */
const SLIDE_ALLOWED_TAGS = [
  // block and inline markdown
  "section",
  "div",
  "p",
  "br",
  "hr",
  "strong",
  "em",
  "del",
  "code",
  "pre",
  "span",
  "blockquote",
  "ul",
  "ol",
  "li",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "table",
  "thead",
  "tbody",
  "tr",
  "th",
  "td",
  "img",
  "figure",
  "figcaption",
  "a",
  "kbd",
  "sup",
  "sub",
  // KaTeX MathML
  "math",
  "semantics",
  "annotation",
  "mrow",
  "mi",
  "mn",
  "mo",
  "mtext",
  "ms",
  "mspace",
  "mfrac",
  "msqrt",
  "mroot",
  "msup",
  "msub",
  "msubsup",
  "munder",
  "mover",
  "munderover",
  "mtable",
  "mtr",
  "mtd",
  "mstyle",
  "mpadded",
  "mphantom",
  "menclose",
  "mline",
  // KaTeX SVG (the HTML build draws radicals, delimiters and stretchy glyphs)
  "svg",
  "g",
  "path",
  "line",
  "rect",
  "use",
  "defs",
  "symbol",
];

/** Attributes the rendered slides legitimately carry. */
const SLIDE_ALLOWED_ATTR = [
  "class",
  "id",
  "style",
  "title",
  "alt",
  "src",
  "href",
  "target",
  "rel",
  "type",
  "start",
  "checked",
  "disabled",
  "aria-label",
  "aria-hidden",
  // Marp slide metadata
  "data-marpit-fragment",
  "data-marpit-fragments",
  "data-marpit-pagination",
  "data-marpit-pagination-total",
  "data-marpit-svg",
  "data-theme",
  "data-class",
  "data-paginate",
  "data-size",
  // SVG (KaTeX output)
  "viewBox",
  "width",
  "height",
  "xmlns",
  "preserveAspectRatio",
  "display",
  "d",
  "fill",
  "fill-rule",
  "stroke",
  "stroke-width",
  "x",
  "y",
  "x1",
  "y1",
  "x2",
  "y2",
  "transform",
  "aria-hidden",
  // MathML (KaTeX output)
  "mathvariant",
  "encoding",
  "mathsize",
  "stretchy",
  "lspace",
  "rspace",
];

/** The slide-scoped DOMPurify configuration (exported for the guard test). */
export const SLIDES_SANITIZE_CONFIG: Config = {
  ALLOWED_TAGS: SLIDE_ALLOWED_TAGS,
  ALLOWED_ATTR: SLIDE_ALLOWED_ATTR,
};

/**
 * Sanitize one rendered slide's HTML with the slide-scoped config. The global
 * config in `markdown.ts` is not consulted or modified.
 */
export function sanitizeSlideHtml(html: string): string {
  return DOMPurify.sanitize(html, SLIDES_SANITIZE_CONFIG);
}

/** Sanitize each slide in a deck, preserving order. */
export function sanitizeSlides(slides: string[]): string[] {
  return slides.map(sanitizeSlideHtml);
}
