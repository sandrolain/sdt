import DOMPurify, { type Config } from "dompurify";

/**
 * The diagram-scoped sanitizer.
 *
 * The global markdown allowlist in `markdown.ts` deliberately excludes `svg`
 * and `style`; a rendered diagram needs both. Pele emits one SVG per diagram
 * whose colours are CSS-variable references (`var(--pele-…)`) declared in an
 * inline `<style>` block, so stripping either turns every diagram into black
 * shapes on an unstyled background. This config allows exactly the rendered
 * diagram surface and still strips script and event handlers; the global
 * allowlist is never widened (the decision-0022 pattern from the slide lane).
 *
 * `style` is allowed as an **attribute** (Pele's root element maps its
 * `--pele-*` names onto private `--_*` properties inline) but not as a
 * `<style>` tag: no rendered diagram carries one.
 */
const DIAGRAM_ALLOWED_TAGS = [
  // the diagram root and its containers
  "svg",
  "g",
  "defs",
  "title",
  "desc",
  // shapes
  "path",
  "rect",
  "circle",
  "ellipse",
  "polygon",
  "polyline",
  "line",
  "image",
  "linearGradient",
  "stop",
  // text
  "text",
  "tspan",
  "br",
  // rich content (icon labels, resolved math)
  "foreignObject",
  "div",
  "span",
];

/** Attributes the rendered diagrams legitimately carry. */
const DIAGRAM_ALLOWED_ATTR = [
  "xmlns",
  "class",
  "id",
  "style",
  "role",
  "aria-roledescription",
  "aria-hidden",
  "aria-labelledby",
  "aria-describedby",
  "viewBox",
  "preserveAspectRatio",
  "width",
  "height",
  "x",
  "y",
  "x1",
  "y1",
  "x2",
  "y2",
  "cx",
  "cy",
  "r",
  "rx",
  "ry",
  "d",
  "points",
  "transform",
  "fill",
  "fill-opacity",
  "fill-rule",
  "stroke",
  "stroke-width",
  "stroke-linecap",
  "stroke-linejoin",
  "stroke-dasharray",
  "opacity",
  "color",
  "overflow",
  "pointer-events",
  "font-family",
  "font-size",
  "font-style",
  "font-weight",
  "text-anchor",
  "text-decoration",
  "baseline-shift",
  "data-id",
  "data-icon",
  "data-href",
  "xml:space",
  // gradients and images
  "gradientUnits",
  "offset",
  "stop-color",
  "href",
  "target",
  "rel",
];

/** The diagram-scoped DOMPurify configuration (exported for the guard test). */
export const DIAGRAM_SANITIZE_CONFIG: Config = {
  ALLOWED_TAGS: DIAGRAM_ALLOWED_TAGS,
  ALLOWED_ATTR: DIAGRAM_ALLOWED_ATTR,
};

/**
 * Sanitize one rendered diagram's SVG. The global config in `markdown.ts` is
 * not consulted or modified.
 */
export function sanitizeDiagramSvg(svg: string): string {
  return DOMPurify.sanitize(svg, DIAGRAM_SANITIZE_CONFIG) as unknown as string;
}
