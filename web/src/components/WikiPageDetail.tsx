import { useParams } from "react-router-dom";
import { isMapPath } from "../lib/documentModes";
import { useDoc } from "../lib/useDoc";
import { DocumentView } from "./DocumentView";

/** Wiki page detail at #/wiki/*, rendering context/wiki/<id>.md. */
export function WikiPageDetail() {
  const params = useParams();
  const id = params["*"] ?? "";
  const path = `context/wiki/${id}.md`;
  const { doc, error, loading } = useDoc(path);

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
      </header>
      <DocumentView
        path={doc.path}
        frontmatter={doc.frontmatter}
        markdown={doc.markdown}
        isMap={isMapPath(doc.path)}
      />
    </article>
  );
}
