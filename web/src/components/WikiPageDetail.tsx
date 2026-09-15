import { lazy, Suspense, useState } from "react";
import { useParams } from "react-router-dom";
import { isMapPath } from "../lib/documentModes";
import { useDoc } from "../lib/useDoc";
import { DocumentView } from "./DocumentView";

const MindmapView = lazy(() => import("./MindmapView").then((m) => ({ default: m.MindmapView })));

type WikiView = "document" | "mindmap";

/** Wiki page detail at #/wiki/*, rendering context/wiki/<id>.md. */
export function WikiPageDetail() {
  const params = useParams();
  const id = params["*"] ?? "";
  const path = `context/wiki/${id}.md`;
  const { doc, error, loading } = useDoc(path);
  const [view, setView] = useState<WikiView>("document");

  if (error) return <p className="content__empty">wiki error: {error}</p>;
  if (loading || !doc) return <p className="content__empty">Loading {id}…</p>;
  if (!("markdown" in doc)) return <p className="content__empty">not a markdown wiki page</p>;

  const title =
    /^title:\s*(.+)$/m
      .exec(doc.frontmatter)?.[1]
      ?.trim()
      .replace(/^["']|["']$/g, "") || id;

  return (
    <article>
      <header className="doc-header">
        <div className="doc-header__path">{doc.path}</div>
        <h1 className="doc-header__title">{title}</h1>
        <div className="doc-view-toggle" role="group" aria-label="Page view">
          <button
            type="button"
            className={`doc-mode${view === "document" ? " is-active" : ""}`}
            aria-pressed={view === "document"}
            onClick={() => setView("document")}
          >
            Document
          </button>
          <button
            type="button"
            className={`doc-mode${view === "mindmap" ? " is-active" : ""}`}
            aria-pressed={view === "mindmap"}
            onClick={() => setView("mindmap")}
          >
            Mindmap
          </button>
        </div>
      </header>
      {view === "document" ? (
        <DocumentView
          path={doc.path}
          frontmatter={doc.frontmatter}
          markdown={doc.markdown}
          isMap={isMapPath(doc.path)}
        />
      ) : (
        <Suspense fallback={<p className="content__empty">Loading mindmap…</p>}>
          <MindmapView markdown={doc.markdown} basePath={doc.path} title={title} />
        </Suspense>
      )}
    </article>
  );
}
