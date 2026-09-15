import { isCanvas, type CanvasResponse, type DocResponse } from "../lib/api";
import { useDoc } from "../lib/useDoc";
import { DocumentView } from "./DocumentView";

interface DocDetailProps {
  /** corpus-relative path from the URL, e.g. context/wiki/foo.md */
  path: string;
}

export function DocDetail({ path }: DocDetailProps) {
  const { doc, error, loading } = useDoc(path);

  if (error) return <p className="content__empty">doc error: {error}</p>;
  if (loading || !doc) return <p className="content__empty">Loading {path}…</p>;

  if (isCanvas(doc)) {
    return (
      <article>
        <header className="doc-header">
          <div className="doc-header__path">{doc.path}</div>
          <h1 className="doc-header__title">{heading(doc)}</h1>
        </header>
        <pre className="doc-markdown">{JSON.stringify(doc.canvas, null, 2)}</pre>
      </article>
    );
  }

  return (
    <article>
      <header className="doc-header">
        <div className="doc-header__path">{doc.path}</div>
        <h1 className="doc-header__title">{heading(doc)}</h1>
      </header>
      <DocumentView path={doc.path} frontmatter={doc.frontmatter} markdown={doc.markdown} />
    </article>
  );
}

function heading(doc: DocResponse | CanvasResponse): string {
  if (!("frontmatter" in doc)) return doc.path.split("/").pop() ?? doc.path;
  const title = /^title:\s*(.+)$/m.exec(doc.frontmatter);
  if (title) return title[1].trim().replace(/^["']|["']$/g, "");
  const base = doc.path.split("/").pop() ?? doc.path;
  return base.endsWith(".md") ? base.slice(0, -3) : base;
}
