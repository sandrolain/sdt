import { lazy, Suspense, useEffect, useMemo, useRef, useState, type MouseEvent } from "react";
import { useSearchParams } from "react-router-dom";
import { setActiveSection } from "../lib/activeSection";
import { lineNumbers } from "../lib/codeLines";
import {
  defaultMode,
  isDocumentMode,
  isMapPath,
  MAP_ICON,
  modesFor,
  type DocumentMode,
} from "../lib/documentModes";
import { Icon } from "../lib/icon";
import { renderMath } from "../lib/katexRender";
import { highlightMarkdown, renderMarkdown } from "../lib/markdown";
import { renderMermaid } from "../lib/mermaidRender";
import { useOpenDocsOptional } from "../lib/openDocsContext";
import { consumeSectionRequest, useSectionRequest } from "../lib/sectionRequests";
import { useFindInDoc } from "../lib/findInDocStore";
import { fallbackTitle, frontmatterTitle } from "../lib/titles";
import { useActiveHeading } from "../lib/useActiveHeading";
import { loadWikiIndex } from "../lib/wikiIndexLoader";
import type { WikiIndex } from "../lib/wikiLinks";
import { FindInDoc } from "./FindInDoc";
import { HoverPreview } from "./HoverPreview";

const MindmapView = lazy(() => import("./MindmapView").then((m) => ({ default: m.MindmapView })));

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
function copyCodeBlock(button: Element): void {
  const code = button.closest(".md-code-block")?.querySelector("code");
  if (!code || !navigator.clipboard) return;
  void navigator.clipboard.writeText(code.textContent ?? "").then(() => {
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

/** Code / Render / Map surface shared by the docs and wiki detail routes. */
export function DocumentView({ path, frontmatter, markdown, isMap }: DocumentViewProps) {
  const [params, setParams] = useSearchParams();
  const mapDoc = isMap ?? isMapPath(path);
  const modes = useMemo(() => modesFor(mapDoc), [mapDoc]);
  const paramMode = params.get("view");
  const mode: DocumentMode =
    isDocumentMode(paramMode) && modes.some((m) => m.id === paramMode)
      ? paramMode
      : defaultMode(mapDoc);
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
    () => (mode === "render" ? renderMarkdown(markdown, { basePath: path, wikiIndex: index }) : ""),
    [mode, markdown, path, index],
  );
  const code = useMemo(
    () => (mode === "code" ? highlightMarkdown(markdown) : ""),
    [mode, markdown],
  );
  const title = useMemo(
    () => frontmatterTitle(frontmatter) || fallbackTitle(path),
    [frontmatter, path],
  );

  const docsApi = useOpenDocsOptional();
  const renderedRef = useRef<HTMLDivElement | null>(null);
  const [articleEl, setArticleEl] = useState<HTMLElement | null>(null);
  const { open: findOpen } = useFindInDoc();

  // publish the heading in view so the Sections sidebar can highlight it
  useActiveHeading(renderedRef, path, mode === "render");

  // render $…$/$$…$$ math placeholders (lazy KaTeX chunk) after each render
  useEffect(() => {
    if (mode !== "render") return;
    void renderMath(renderedRef.current);
  }, [mode, html]);

  // render mermaid placeholders (lazy chunk, theme-aware) after each render
  useEffect(() => {
    if (mode !== "render") return;
    void renderMermaid(renderedRef.current);
  }, [mode, html]);

  // a Sections click switches to render mode (if needed) then scrolls there
  const sectionRequest = useSectionRequest();
  useEffect(() => {
    if (!sectionRequest || sectionRequest.path !== path) return;
    if (mode !== "render") {
      const nextParams = new URLSearchParams(params);
      nextParams.set("view", "render");
      setParams(nextParams, { replace: true });
      return;
    }
    const root = renderedRef.current;
    const target = root
      ? Array.from(root.querySelectorAll<HTMLElement>("h1,h2,h3,h4,h5,h6")).find(
          (h) => (h.dataset.heading ?? h.textContent ?? "").trim() === sectionRequest.text,
        )
      : undefined;
    target?.scrollIntoView({ behavior: "smooth", block: "start" });
    setActiveSection(path, sectionRequest.text);
    consumeSectionRequest(sectionRequest);
  }, [sectionRequest, mode, path, params, setParams]);

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
      </div>
      {mode === "code" && (
        <div className="doc-code-wrap">
          <pre className="doc-code__gutter" aria-hidden="true">
            {lineNumbers(markdown)}
          </pre>
          <pre className="doc-code">
            <code className="hljs" dangerouslySetInnerHTML={{ __html: code }} />
          </pre>
        </div>
      )}
      {mode === "render" && (
        <>
          <div
            className="doc-rendered"
            ref={renderedRef}
            onClick={onRenderedClick}
            dangerouslySetInnerHTML={{ __html: html }}
          />
          <HoverPreview />
        </>
      )}
      {mode === "map" && (
        <Suspense fallback={<p className="content__empty">Loading map…</p>}>
          <MindmapView markdown={markdown} basePath={path} title={title} />
        </Suspense>
      )}
    </article>
  );
}
