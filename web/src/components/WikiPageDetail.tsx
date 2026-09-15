import { lazy, Suspense, useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import { isMapPath } from "../lib/documentModes";
import { useDoc } from "../lib/useDoc";
import { displayTitle, frontmatterTitle } from "../lib/titles";
import { DocumentView } from "./DocumentView";
import { DocMetaPanel } from "./DocMetaPanel";
import { DockLayout, type DockPanelDef } from "./DockLayout";
import { Icon } from "../lib/icon";
import type { DocResponse } from "../lib/api";

const MindmapView = lazy(() => import("./MindmapView").then((m) => ({ default: m.MindmapView })));

type WikiView = "document" | "mindmap";

/** Wiki page detail at #/wiki/*, rendering context/wiki/<id>.md. */
export function WikiPageDetail() {
  const params = useParams();
  const id = params["*"] ?? "";
  const path = `context/wiki/${id}.md`;
  const { doc, error, loading } = useDoc(path);

  if (error) return <p className="content__empty">wiki error: {error}</p>;
  if (loading || !doc) return <p className="content__empty">Loading {id}…</p>;
  if (!("markdown" in doc)) return <p className="content__empty">not a markdown wiki page</p>;

  const title = displayTitle({
    title: frontmatterTitle(doc.frontmatter),
    markdown: doc.markdown,
    path,
  });

  return <WikiDetail doc={doc} id={id} title={title} />;
}

function WikiDetail({ doc, id, title }: { doc: DocResponse; id: string; title: string }) {
  const [view, setView] = useState<WikiView>("document");

  const panels = useMemo<DockPanelDef[]>(
    () => [
      {
        id: "content",
        title: "Page",
        icon: "description",
        minWidth: 320,
        render: () => (
          <article className="wiki-article">
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
                  <Icon name="description" />
                  Document
                </button>
                <button
                  type="button"
                  className={`doc-mode${view === "mindmap" ? " is-active" : ""}`}
                  aria-pressed={view === "mindmap"}
                  onClick={() => setView("mindmap")}
                >
                  <Icon name="account_tree" />
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
        ),
      },
      {
        id: "meta",
        title: "Metadata",
        icon: "info",
        minWidth: 190,
        maxWidth: 460,
        render: () => <DocMetaPanel doc={doc} relatedId={id} />,
      },
    ],
    [doc, id, title, view],
  );

  return <DockLayout storageKey="detail" panels={panels} />;
}
