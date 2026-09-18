import DOMPurify, { type Config } from "dompurify";
import hljs from "highlight.js/lib/common";
import { Marked, type Tokens } from "marked";
import markedFootnote from "marked-footnote";
import { parseCodeInfo, wrapHighlightedLines } from "./codeLines";
import { deflistExtension } from "./deflistExtension";
import { FEATURES } from "./features";
import { stripLeadingH1 } from "./headings";
import { imageSrc } from "./images";
import { mathExtensions } from "./mathExtension";
import { docHref, resolveDocPath, rewriteWikiLinks, type WikiIndex } from "./wikiLinks";

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
    "figure",
    "figcaption",
    "dl",
    "dt",
    "dd",
    "section",
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
    .replace(/&(amp;)?/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

/** GitHub-style callout labels keyed by the `[!TYPE]` marker. */
const CALLOUTS: Record<string, string> = {
  note: "Note",
  tip: "Tip",
  important: "Important",
  warning: "Warning",
  caution: "Caution",
};

/** Lowercase dash slug for heading anchors. */
function slugify(text: string): string {
  const slug = text
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[^\w\s-]/g, "")
    .trim()
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-");
  return slug || "section";
}

/** Build a marked instance whose link renderer knows the base path. */
function buildMarked(basePath?: string): Marked {
  // per-document slug counter so duplicate headings get unique ids
  const slugs = new Map<string, number>();
  const uniqueSlug = (text: string): string => {
    const base = slugify(text);
    const count = slugs.get(base) ?? 0;
    slugs.set(base, count + 1);
    return count === 0 ? base : `${base}-${count}`;
  };

  const marked = new Marked({
    gfm: true,
    breaks: false,
    extensions: [...(FEATURES.katex ? mathExtensions() : []), ...deflistExtension()],
    renderer: {
      heading(this: { parser: { parseInline(tokens: unknown): string } }, token: Tokens.Heading) {
        const text = this.parser.parseInline(token.tokens);
        const plain = text.replace(/<[^>]+>/g, "").trim();
        const id = uniqueSlug(plain);
        return [
          `<h${token.depth} id="${id}" data-heading="${escapeHtml(plain)}">`,
          text,
          `<button type="button" class="md-anchor" data-anchor="${id}" aria-label="Copy link to section" title="Copy section link"><span class="ms-icon" aria-hidden="true">link</span></button>`,
          `</h${token.depth}>`,
        ].join("");
      },
      blockquote(
        this: { parser: { parse(tokens: unknown): string } },
        token: Tokens.Blockquote,
      ): string {
        let inner = this.parser.parse(token.tokens);
        const match = /^\s*<p>\s*\[!([A-Za-z]+)\]\s*(?:<br\s*\/?>)?\s*/i.exec(inner);
        const label = match ? CALLOUTS[match[1].toLowerCase()] : undefined;
        if (!match || !label) return `<blockquote>${inner}</blockquote>`;
        const type = match[1].toLowerCase();
        inner = inner.slice(match[0].length);
        // re-open a <p> so the sliced opening tag stays balanced
        return [
          `<div class="md-callout md-callout--${type}">`,
          `<p class="md-callout__title">${label}</p>`,
          `<div class="md-callout__body"><p>${inner}</div>`,
          "</div>",
        ].join("");
      },
      code({ text, lang }: Tokens.Code): string {
        if (FEATURES.mermaid && lang === "mermaid") {
          // rendered client-side by mermaidRender; the source travels base64-free
          // (URI-encoded) so DOMPurify keeps the data attribute untouched
          return `<div class="md-mermaid" data-src="${escapeHtml(encodeURIComponent(text))}"></div>`;
        }
        const { language, highlighted } = parseCodeInfo(lang);
        const known = language && hljs.getLanguage(language);
        const highlightLang = known ? language : undefined;
        const body = wrapHighlightedLines(highlightCode(text, highlightLang), highlighted);
        return [
          '<div class="md-code-block">',
          language
            ? `<span class="md-code__lang">${escapeHtml(language)}</span>`
            : '<span class="md-code__lang md-code__lang--plain">text</span>',
          '<button type="button" class="md-code__wrap" aria-label="Toggle line wrapping" title="Toggle line wrapping">',
          '<span class="ms-icon" aria-hidden="true">wrap_text</span>',
          "</button>",
          '<button type="button" class="md-code__copy" aria-label="Copy code" title="Copy code">',
          '<span class="ms-icon" aria-hidden="true">content_copy</span>',
          "</button>",
          `<pre class="md-code"><code class="hljs language-${escapeHtml(known ? language : "plaintext")}">${body}</code></pre>`,
          "</div>",
        ].join("");
      },
      link({ href, title, text }: Tokens.Link): string {
        const attrs: string[] = [];
        let target = href;
        if (/^https?:\/\//i.test(href)) {
          attrs.push('target="_blank" rel="noopener noreferrer"');
        } else if (!href.startsWith("/")) {
          // relative links resolve against the document; app routes (/docs,
          // /wiki) and fragment links are already absolute
          target = docHref(resolveDocPath(href, basePath));
        }
        if (title) attrs.push(`title="${escapeHtml(title)}"`);
        const attrStr = attrs.length > 0 ? ` ${attrs.join(" ")}` : "";
        return `<a href="${escapeHtml(target)}"${attrStr}>${escapeHtml(text)}</a>`;
      },
      table(
        this: { parser: { parseInline(tokens: unknown): string } },
        token: Tokens.Table,
      ): string {
        const alignClass = (index: number) => {
          const align = token.align?.[index];
          return align ? ` class="md-align-${align}"` : "";
        };
        const cell = (c: Tokens.TableCell, tag: "th" | "td", index: number) =>
          `<${tag}${alignClass(index)}>${this.parser.parseInline(c.tokens)}</${tag}>`;
        const head = `<thead><tr>${token.header
          .map((c, i) => cell(c, "th", i))
          .join("")}</tr></thead>`;
        const body = `<tbody>${token.rows
          .map((row) => `<tr>${row.map((c, i) => cell(c, "td", i)).join("")}</tr>`)
          .join("")}</tbody>`;
        return `<div class="md-table-wrap"><table>${head}${body}</table></div>`;
      },
      image({ href, title, text }: Tokens.Image): string {
        const src = imageSrc(href, basePath);
        const titleAttr = title ? ` title="${escapeHtml(title)}"` : "";
        const img = `<img src="${escapeHtml(src)}" alt="${escapeHtml(text ?? "")}"${titleAttr} />`;
        // a title becomes a visible caption (figure/figcaption)
        return title ? `<figure>${img}<figcaption>${escapeHtml(title)}</figcaption></figure>` : img;
      },
    },
  });
  marked.use(markedFootnote());
  return marked;
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
