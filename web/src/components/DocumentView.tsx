import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { highlightMarkdown, renderMarkdown } from "../lib/markdown";
import { parseOutline, type OutlineItem } from "../lib/outline";
import {
  defaultMode,
  DOCUMENT_MODES,
  isDocumentMode,
  isMapPath,
  type DocumentMode,
} from "../lib/documentModes";
import { loadWikiIndex } from "../lib/wikiIndexLoader";
import type { WikiIndex } from "../lib/wikiLinks";

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
  const outline = useMemo(() => (mode === "map" ? parseOutline(markdown) : []), [mode, markdown]);

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
            {m.label}
          </button>
        ))}
        {mapDoc && <span className="doc-badge doc-badge--map">map</span>}
      </div>
      {frontmatter && <pre className="doc-frontmatter">{frontmatter}</pre>}
      {mode === "code" && (
        <pre className="doc-code">
          <code className="hljs" dangerouslySetInnerHTML={{ __html: code }} />
        </pre>
      )}
      {mode === "render" && (
        <div className="doc-rendered" dangerouslySetInnerHTML={{ __html: html }} />
      )}
      {mode === "map" && <Outline items={outline} />}
    </article>
  );
}

function Outline({ items }: { items: OutlineItem[] }) {
  if (items.length === 0) {
    return <p className="content__empty">No headings or lists to map.</p>;
  }
  return (
    <ul className="outline" role="list">
      {items.map((item, i) => (
        <li
          key={i}
          className={`outline__item outline__item--${item.kind} outline__depth-${item.depth}`}
        >
          <span className="outline__text">{item.text}</span>
          {item.children.length > 0 && <Outline items={item.children} />}
        </li>
      ))}
    </ul>
  );
}
