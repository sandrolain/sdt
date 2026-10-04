import { lazy, Suspense, useEffect, useMemo, useRef, useState, type MouseEvent } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { activeSectionKey, setActiveSection } from "../lib/activeSection";
import { codeBlockText, lineNumbers } from "../lib/codeLines";
import {
  defaultMode,
  isDocumentMode,
  isMapPath,
  isMermaidPath,
  isSlidePath,
  MAP_ICON,
  MERMAID_ICON,
  modesFor,
  SLIDES_ICON,
  type DocumentMode,
} from "../lib/documentModes";
import { Icon } from "../lib/icon";
import { normalizeHeadingText } from "../lib/headings";
import { renderMath } from "../lib/katexRender";
import {
  highlightCode,
  highlightMarkdown,
  highlightYaml,
  renderMarkdown,
  slugify,
} from "../lib/markdown";
import { renderMermaid } from "../lib/mermaidRender";
import { useOpenDocsOptional } from "../lib/openDocsContext";
import { flushReading, recordReading, restoreScrollOffset } from "../lib/readingState";
import {
  consumeSectionRequest,
  getSectionRequest,
  requestSection,
  useSectionRequest,
} from "../lib/sectionRequests";
import { resetFindQuery, useFindInDoc } from "../lib/findInDocStore";
import { fallbackTitle, frontmatterTitle } from "../lib/titles";
import { useActiveHeading } from "../lib/useActiveHeading";
import { loadWikiIndex } from "../lib/wikiIndexLoader";
import type { WikiIndex } from "../lib/wikiLinks";
import { DocProgress } from "./DocProgress";
import { FindInDoc } from "./FindInDoc";
import { HoverPreview } from "./HoverPreview";

const MindmapView = lazy(() => import("./MindmapView").then((m) => ({ default: m.MindmapView })));
const SlidesView = lazy(() => import("./SlidesView").then((m) => ({ default: m.SlidesView })));

/** Serialize a rendered diagram's SVG and download it. */
function downloadDiagramSvg(button: Element): void {
  const svg = button.closest(".md-mermaid")?.querySelector("svg");
  if (!svg) return;
  const markup = new XMLSerializer().serializeToString(svg);
  const url = URL.createObjectURL(new Blob([markup], { type: "image/svg+xml" }));
  const link = document.createElement("a");
  link.href = url;
  link.download = "diagram.svg";
  link.click();
  URL.revokeObjectURL(url);
}

/** Copy a deep link to a heading, flash the anchor and reveal the section. */
function copyAnchor(button: Element): void {
  const id = button.getAttribute("data-anchor") ?? "";
  if (!id) return;
  const url = `${window.location.origin}${window.location.pathname}#${id}`;
  if (navigator.clipboard) void navigator.clipboard.writeText(url);
  window.history.replaceState(null, "", `#${id}`);
  document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
  button.classList.add("is-copied");
  window.setTimeout(() => button.classList.remove("is-copied"), 1200);
}

/** Corpus path behind a rendered docs/wiki link, or null for non-document links. */
function docsTargetFromHref(href: string): string | null {
  const [pathname] = href.split(/[?#]/);
  if (pathname.startsWith("/docs/")) return decodeURIComponent(pathname.slice("/docs/".length));
  if (pathname.startsWith("/wiki/")) {
    const id = pathname.slice("/wiki/".length);
    if (!id || id === "graph" || id === "board") return null;
    return `context/wiki/${decodeURIComponent(id)}.md`;
  }
  return null;
}

/** Copy the sibling `<code>` text and flash a success glyph on the button. */
/**
 * Heading text for a fragment anchor: by element id first, then by comparing the
 * slugified heading text. The server slug and the renderer slug agree for plain
 * headings; the fallback covers a heading with punctuation.
 */
function resolveHeadingText(root: HTMLElement | null, anchor: string): string | null {
  if (!root) return null;
  const byId = root.querySelector<HTMLElement>(`[id="${CSS.escape(anchor)}"]`);
  if (byId) return normalizeHeadingText(byId.dataset.heading ?? byId.textContent ?? "") || null;
  for (const heading of root.querySelectorAll<HTMLElement>("h1,h2,h3,h4,h5,h6")) {
    const text = normalizeHeadingText(heading.dataset.heading ?? heading.textContent ?? "");
    if (text && slugify(text) === anchor) return text;
  }
  return null;
}

function copyCodeBlock(button: Element): void {
  const code = button.closest(".md-code-block")?.querySelector("code");
  if (!code || !navigator.clipboard) return;
  void navigator.clipboard.writeText(codeBlockText(code)).then(() => {
    const glyph = button.querySelector(".ms-icon");
    const previous = glyph?.textContent ?? "content_copy";
    if (glyph) glyph.textContent = "check";
    button.setAttribute("aria-label", "Copied");
    window.setTimeout(() => {
      if (glyph) glyph.textContent = previous;
      button.setAttribute("aria-label", "Copy code");
    }, 1200);
  });
}

interface DocumentViewProps {
  path: string;
  frontmatter?: string;
  markdown: string;
  /** server isMap hint; falls back to the `.map.md` path convention */
  isMap?: boolean;
}

/** Placeholder consumed by `renderMermaid` for a standalone `.mmd` document. */
function mermaidPlaceholder(source: string): string {
  return `<div class="md-mermaid" data-src="${encodeURIComponent(source)}"></div>`;
}

/** Code / Render / Map / Mermaid surface shared by the docs and wiki detail routes. */
export function DocumentView({ path, frontmatter, markdown, isMap }: DocumentViewProps) {
  const [params, setParams] = useSearchParams();
  const location = useLocation();
  const navigate = useNavigate();
  const mapDoc = isMap ?? isMapPath(path);
  const mermaidDoc = isMermaidPath(path);
  const slideDoc = isSlidePath(path);
  const shape = useMemo(
    () => ({ isMap: mapDoc, isMermaid: mermaidDoc, isSlide: slideDoc }),
    [mapDoc, mermaidDoc, slideDoc],
  );
  const modes = useMemo(() => modesFor(shape), [shape]);
  const paramMode = params.get("view");
  const mode: DocumentMode =
    isDocumentMode(paramMode) && modes.some((m) => m.id === paramMode)
      ? paramMode
      : defaultMode(shape);
  const [index, setIndex] = useState<WikiIndex | undefined>(undefined);

  useEffect(() => {
    let alive = true;
    loadWikiIndex()
      .then((ix) => {
        if (alive) setIndex(ix);
      })
      .catch(() => {
        // link resolution degrades to broken-link spans; the doc still renders
      });
    return () => {
      alive = false;
    };
  }, []);

  const setMode = (next: DocumentMode) => {
    const nextParams = new URLSearchParams(params);
    nextParams.set("view", next);
    setParams(nextParams, { replace: true });
  };

  const html = useMemo(
    () =>
      mode === "render"
        ? renderMarkdown(markdown, { basePath: path, wikiIndex: index })
        : mode === "mermaid"
          ? mermaidPlaceholder(markdown)
          : "",
    [mode, markdown, path, index],
  );
  // whole-file source: the frontmatter (when present) plus the body, so Code
  // mode shows the file and the gutter numbers its real lines
  const codeSource = useMemo(
    () => (mermaidDoc ? markdown : `${frontmatter ?? ""}${markdown}`),
    [mermaidDoc, frontmatter, markdown],
  );
  const code = useMemo(
    () =>
      mode === "code"
        ? mermaidDoc
          ? highlightCode(markdown, "mermaid")
          : highlightYaml(frontmatter ?? "") + highlightMarkdown(markdown)
        : "",
    [mode, markdown, mermaidDoc, frontmatter],
  );
  const title = useMemo(
    () => frontmatterTitle(frontmatter) || fallbackTitle(path),
    [frontmatter, path],
  );

  const docsApi = useOpenDocsOptional();
  const renderedRef = useRef<HTMLDivElement | null>(null);
  // the rendered node as state too: DocProgress needs it during render, and a
  // ref alone would leave the first pass with null
  const [renderedEl, setRenderedEl] = useState<HTMLDivElement | null>(null);
  const [articleEl, setArticleEl] = useState<HTMLElement | null>(null);
  const { open: findOpen } = useFindInDoc();

  // switching the open document empties the find query while the match options
  // and the open bar survive; a mode switch within one document keeps it.
  useEffect(() => {
    resetFindQuery();
  }, [path]);

  // publish the heading in view so the Sections sidebar can highlight it
  useActiveHeading(renderedRef, path, mode === "render");

  // a deep link (#anchor from a search hit) resolves to a heading and reuses the
  // Sections path, so a hit lands where it matched rather than at the top. The
  // fragment belongs to the route, so it only applies to the document it names;
  // `consumed` records the anchor already turned into a request, which keeps a
  // later re-render from requesting it again.
  const routeAnchor =
    location.pathname === `/docs/${path}`
      ? decodeURIComponent(location.hash.replace(/^#/, "")) || null
      : null;
  const [consumed, setConsumed] = useState<string | null>(null);
  const pendingAnchor = routeAnchor && routeAnchor !== consumed ? routeAnchor : null;

  // reading position: restore where this document was left, record where it is.
  // The scroller is `.doc-rendered`, not the article; `articleEl` is in the deps
  // because it changes with the mount that populates `renderedRef`.
  const readingMode = mode === "render" || mode === "mermaid";
  useEffect(() => {
    const root = renderedRef.current;
    if (!root || !readingMode) return;
    // an explicit deep link wins over the remembered offset
    if (pendingAnchor || getSectionRequest()?.path === path) return;
    restoreScrollOffset(root, path);
  }, [articleEl, path, html, readingMode, pendingAnchor]);

  useEffect(() => {
    const root = renderedRef.current;
    if (!root || !readingMode) return;
    let frame = 0;
    const onScroll = () => {
      if (frame) return;
      frame = requestAnimationFrame(() => {
        frame = 0;
        recordReading(path, root.scrollTop, activeSectionKey(path));
      });
    };
    root.addEventListener("scroll", onScroll, { passive: true });
    return () => {
      root.removeEventListener("scroll", onScroll);
      if (frame) cancelAnimationFrame(frame);
      flushReading();
    };
  }, [articleEl, path, readingMode]);

  // render $…$/$$…$$ math placeholders (lazy KaTeX chunk) after each render
  useEffect(() => {
    if (mode !== "render") return;
    void renderMath(renderedRef.current);
  }, [mode, html]);

  // render mermaid placeholders (lazy chunk, theme-aware) after each render
  useEffect(() => {
    if (mode !== "render" && mode !== "mermaid") return;
    void renderMermaid(renderedRef.current);
  }, [mode, html]);

  // a Sections click switches to render mode (if needed) then scrolls there
  const sectionRequest = useSectionRequest();
  useEffect(() => {
    if (!sectionRequest || sectionRequest.path !== path) return;
    if (mode !== "render") {
      const nextParams = new URLSearchParams(params);
      nextParams.set("view", "render");
      // one navigation, hash included: a deep link must survive the mode switch
      navigate(
        { pathname: location.pathname, search: nextParams.toString(), hash: location.hash },
        { replace: true },
      );
      return;
    }
    const root = renderedRef.current;
    const target = root
      ? Array.from(root.querySelectorAll<HTMLElement>("h1,h2,h3,h4,h5,h6")).find(
          (h) =>
            normalizeHeadingText(h.dataset.heading ?? h.textContent ?? "") ===
            normalizeHeadingText(sectionRequest.text),
        )
      : undefined;
    target?.scrollIntoView({ behavior: "smooth", block: "start" });
    setActiveSection(path, sectionRequest.text);
    consumeSectionRequest(sectionRequest);
  }, [sectionRequest, mode, path, params, navigate, location.pathname, location.hash]);

  // resolve the deep-link anchor once the rendered headings exist; a deep link
  // opens in render mode, because that is the only surface with headings
  useEffect(() => {
    if (!pendingAnchor) return;
    if (mode !== "render") {
      const nextParams = new URLSearchParams(params);
      nextParams.set("view", "render");
      navigate(
        { pathname: location.pathname, search: nextParams.toString(), hash: location.hash },
        { replace: true },
      );
      return;
    }
    const text = resolveHeadingText(renderedRef.current, pendingAnchor);
    // an anchor no heading answers to: keep the reader at the top, and do not ask
    // again on the next render
    // oxlint-disable-next-line react-hooks/set-state-in-effect
    setConsumed(pendingAnchor);
    if (text) requestSection(path, text);
  }, [
    pendingAnchor,
    consumed,
    mode,
    path,
    html,
    params,
    navigate,
    location.pathname,
    location.hash,
  ]);

  const onRenderedClick = (event: MouseEvent<HTMLDivElement>) => {
    const anchorButton = (event.target as HTMLElement | null)?.closest?.(".md-anchor");
    if (anchorButton) {
      event.preventDefault();
      copyAnchor(anchorButton);
      return;
    }
    const downloadButton = (event.target as HTMLElement | null)?.closest?.(".md-mermaid__download");
    if (downloadButton) {
      event.preventDefault();
      downloadDiagramSvg(downloadButton);
      return;
    }
    const wrapButton = (event.target as HTMLElement | null)?.closest?.(".md-code__wrap");
    if (wrapButton) {
      event.preventDefault();
      wrapButton
        .closest(".md-code-block")
        ?.querySelector("pre.md-code")
        ?.classList.toggle("is-wrapped");
      return;
    }
    const copyButton = (event.target as HTMLElement | null)?.closest?.(".md-code__copy");
    if (copyButton) {
      event.preventDefault();
      copyCodeBlock(copyButton);
      return;
    }
    const anchor = (event.target as HTMLElement | null)?.closest?.("a");
    const href = anchor?.getAttribute("href") ?? "";
    if (!anchor || !docsApi) return;
    const target = docsTargetFromHref(href);
    if (!target) return;
    event.preventDefault();
    docsApi.open(target);
  };

  return (
    <article className="doc-view" ref={setArticleEl}>
      {findOpen && (mode === "render" || mode === "code") && (
        <FindInDoc root={articleEl} mode={mode} />
      )}
      <div className="doc-modes" role="group" aria-label="Document view mode">
        {modes.map((m) => (
          <button
            key={m.id}
            type="button"
            className={`doc-mode${mode === m.id ? " is-active" : ""}`}
            aria-pressed={mode === m.id}
            onClick={() => setMode(m.id)}
          >
            <Icon name={m.icon} />
            {m.label}
          </button>
        ))}
        {mapDoc && <Icon name={MAP_ICON} className="map-icon" label="Map document" />}
        {mermaidDoc && <Icon name={MERMAID_ICON} className="map-icon" label="Mermaid document" />}
        {slideDoc && <Icon name={SLIDES_ICON} className="map-icon" label="Slide deck" />}
      </div>
      {mode === "code" && (
        <div className="doc-code-wrap">
          <pre className="doc-code__gutter" aria-hidden="true">
            {lineNumbers(codeSource)}
          </pre>
          <pre className="doc-code">
            <code className="hljs" dangerouslySetInnerHTML={{ __html: code }} />
          </pre>
        </div>
      )}
      {(mode === "render" || mode === "mermaid") && (
        <>
          <DocProgress container={renderedEl} path={path} />
          <div
            className={mode === "mermaid" ? "doc-rendered doc-rendered--mermaid" : "doc-rendered"}
            ref={(el) => {
              renderedRef.current = el;
              setRenderedEl(el);
            }}
            onClick={onRenderedClick}
            dangerouslySetInnerHTML={{ __html: html }}
          />
          {mode === "render" && <HoverPreview />}
        </>
      )}
      {mode === "map" && (
        <Suspense fallback={<p className="content__empty">Loading map…</p>}>
          <MindmapView markdown={markdown} basePath={path} title={title} />
        </Suspense>
      )}
      {mode === "slides" && (
        <Suspense fallback={<p className="content__empty">Loading slides…</p>}>
          <SlidesView markdown={markdown} />
        </Suspense>
      )}
    </article>
  );
}
