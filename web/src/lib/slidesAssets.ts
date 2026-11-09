import { imageSrc } from "./images";

/**
 * Resolve the assets a rendered slide points at, after sanitization.
 *
 * Marp emits a relative `src` for an inline image and a `background-image`
 * declaration (plus `data-background-image`) on the section for `![bg]`. Both
 * are relative to the deck document, which the SPA cannot resolve; the ordinary
 * markdown renderer rewrites them through `imageSrc`/`/api/file`, and slides
 * need the same treatment. Remote, `data:`, `blob:`, `/api/` and fragment
 * values pass through unchanged.
 */

/** A CSS `url(…)` occurrence, with an optional wrapping quote. */
const CSS_URL_RE = /url\(\s*(["']?)([^"')]+)\1\s*\)/g;

/** Rewrite every `url(…)` in a CSS declaration list. */
function rewriteCssUrls(value: string, basePath?: string): string {
  return value.replace(
    CSS_URL_RE,
    (_match, quote: string, url: string) => `url(${quote}${imageSrc(url, basePath)}${quote})`,
  );
}

/**
 * Rewrite one rendered slide's inline images and background images to corpus
 * URLs. A missing base path leaves the HTML untouched (there is nothing to
 * resolve against).
 */
export function resolveSlideAssets(html: string, basePath?: string): string {
  if (!basePath) return html;
  const template = document.createElement("template");
  template.innerHTML = html;

  for (const img of template.content.querySelectorAll("img[src]")) {
    const src = img.getAttribute("src");
    if (src) img.setAttribute("src", imageSrc(src, basePath));
  }

  for (const section of template.content.querySelectorAll("section[style]")) {
    const style = section.getAttribute("style") ?? "";
    if (style.includes("url(")) section.setAttribute("style", rewriteCssUrls(style, basePath));
  }

  for (const section of template.content.querySelectorAll("section[data-background-image]")) {
    const background = section.getAttribute("data-background-image") ?? "";
    if (background) {
      section.setAttribute("data-background-image", rewriteCssUrls(background, basePath));
    }
  }

  return template.innerHTML;
}
