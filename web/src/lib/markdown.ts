import DOMPurify, { type Config } from "dompurify";
import hljs from "highlight.js/lib/common";
import { Marked, type Tokens } from "marked";
import { docHref, resolveDocPath, rewriteWikiLinks, type WikiIndex } from "./wikiLinks";
import { stripLeadingH1 } from "./headings";

/** Raw-HTML policy: conservative allowlist, everything else is dropped. */
const SANITIZE_CONFIG: Config = {
  ALLOWED_TAGS: [
    "a",
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
    "details",
    "summary",
    "kbd",
    "sup",
    "sub",
    "input",
    "div",
    "button",
  ],
  ALLOWED_ATTR: [
    "href",
    "title",
    "alt",
    "src",
    "class",
    "id",
    "target",
    "rel",
    "type",
    "checked",
    "disabled",
    "start",
    "aria-label",
  ],
};

export interface RenderOptions {
  /** containing document path, for resolving relative `.md` links */
  basePath?: string;
  /** wiki id/title index used to resolve `[[…]]` links */
  wikiIndex?: WikiIndex;
}

/** Boundary markers: inline `[B]`/`[B1]` tokens and `[Bn]: title` lines. */
const BOUNDARY_TITLE_LINE = /^[ \t]*\[B\d*\]:.*(?:\r?\n|$)/gm;
const BOUNDARY_INLINE = /\[B\d*\]/g;

/** Strip XMindMark boundary markers so they never leak into rendered text. */
export function stripBoundaryMarkers(md: string): string {
  return md.replace(BOUNDARY_TITLE_LINE, "").replace(BOUNDARY_INLINE, "");
}

/** Highlight a code block; unknown languages fall back to plaintext. */
export function highlightCode(code: string, lang?: string): string {
  const language = lang && hljs.getLanguage(lang) ? lang : "plaintext";
  try {
    return hljs.highlight(code, { language, ignoreIllegals: true }).value;
  } catch {
    return escapeHtml(code);
  }
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

/** Build a marked instance whose link renderer knows the base path. */
function buildMarked(basePath?: string): Marked {
  return new Marked({
    gfm: true,
    breaks: false,
    renderer: {
      code({ text, lang }: Tokens.Code): string {
        const language = lang && hljs.getLanguage(lang) ? lang : "plaintext";
        const body = highlightCode(text, lang);
        return [
          '<div class="md-code-block">',
          '<button type="button" class="md-code__copy" aria-label="Copy code" title="Copy code">',
          '<span class="ms-icon" aria-hidden="true">content_copy</span>',
          "</button>",
          `<pre class="md-code"><code class="hljs language-${escapeHtml(language)}">${body}</code></pre>`,
          "</div>",
        ].join("");
      },
      link({ href, title, text }: Tokens.Link): string {
        const attrs: string[] = [];
        let target = href;
        if (/^https?:\/\//i.test(href)) {
          attrs.push('target="_blank" rel="noopener noreferrer"');
        } else if (!href.startsWith("#")) {
          target = docHref(resolveDocPath(href, basePath));
        }
        if (title) attrs.push(`title="${escapeHtml(title)}"`);
        const attrStr = attrs.length > 0 ? ` ${attrs.join(" ")}` : "";
        return `<a href="${escapeHtml(target)}"${attrStr}>${escapeHtml(text)}</a>`;
      },
    },
  });
}

/** Render markdown to sanitized HTML: wikilink rewrite, marker strip, sanitize. */
export function renderMarkdown(md: string, opts: RenderOptions = {}): string {
  const prepared = stripBoundaryMarkers(rewriteWikiLinks(stripLeadingH1(md), opts.wikiIndex));
  const raw = buildMarked(opts.basePath).parse(prepared);
  return DOMPurify.sanitize(raw as string, SANITIZE_CONFIG) as unknown as string;
}

/** Highlight raw markdown source (Code mode) as markdown. */
export function highlightMarkdown(md: string): string {
  return highlightCode(md, "markdown");
}
