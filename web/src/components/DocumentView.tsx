import { lazy, Suspense, useEffect, useMemo, useState, type MouseEvent } from "react";
import { useSearchParams } from "react-router-dom";
import { highlightMarkdown, renderMarkdown } from "../lib/markdown";
import {
  defaultMode,
  DOCUMENT_MODES,
  isDocumentMode,
  isMapPath,
  type DocumentMode,
} from "../lib/documentModes";
import { loadWikiIndex } from "../lib/wikiIndexLoader";
import type { WikiIndex } from "../lib/wikiLinks";
import { Icon } from "../lib/icon";
import { fallbackTitle, frontmatterTitle } from "../lib/titles";
import { useOpenDocsOptional } from "../lib/openDocsContext";
import { lineNumbers } from "../lib/codeLines";
import { HoverPreview } from "./HoverPreview";

const MindmapView = lazy(() => import("./MindmapView").then((m) => ({ default: m.MindmapView })));

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
  const paramMode = params.get("view");
  const mode: DocumentMode = isDocumentMode(paramMode) ? paramMode : defaultMode(mapDoc);
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

  const onRenderedClick = (event: MouseEvent<HTMLDivElement>) => {
    const anchor = (event.target as HTMLElement | null)?.closest?.("a");
    const href = anchor?.getAttribute("href") ?? "";
    if (!anchor || !docsApi) return;
    const target = href.startsWith("#/docs/")
      ? href.slice("#/docs/".length)
      : href.startsWith("#/wiki/")
        ? `context/wiki/${href.slice("#/wiki/".length)}.md`
        : null;
    if (!target) return;
    event.preventDefault();
    docsApi.open(decodeURIComponent(target));
  };

  return (
    <article className="doc-view">
      <div className="doc-modes" role="group" aria-label="Document view mode">
        {DOCUMENT_MODES.map((m) => (
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
        {mapDoc && <span className="doc-badge doc-badge--map">map</span>}
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
