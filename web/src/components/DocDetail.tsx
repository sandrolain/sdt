import { useEffect, useState } from "react";
import { fetchDoc, isCanvas, type CanvasResponse, type DocResponse } from "../lib/api";

interface DocDetailProps {
  /** corpus-relative path from the URL, e.g. context/wiki/foo.md */
  path: string;
}

export function DocDetail({ path }: DocDetailProps) {
  const [doc, setDoc] = useState<DocResponse | CanvasResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    fetchDoc(path)
      .then((d) => {
        if (alive) setDoc(d);
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      alive = false;
    };
  }, [path]);

  if (error) return <p className="content__empty">doc error: {error}</p>;
  if (!doc) return <p className="content__empty">Loading {path}…</p>;

  return (
    <article>
      <header className="doc-header">
        <div className="doc-header__path">{doc.path}</div>
        <h1 className="doc-header__title">{heading(doc)}</h1>
      </header>
      {isCanvas(doc) ? (
        <pre className="doc-markdown">{JSON.stringify(doc.canvas, null, 2)}</pre>
      ) : (
        <>
          {doc.frontmatter && <pre className="doc-frontmatter">{doc.frontmatter}</pre>}
          <pre className="doc-markdown">{doc.markdown}</pre>
        </>
      )}
    </article>
  );
}

function heading(doc: DocResponse | CanvasResponse): string {
  if (isCanvas(doc)) return doc.path.split("/").pop() ?? doc.path;
  const title = /^title:\s*(.+)$/m.exec(doc.frontmatter);
  if (title) return title[1].trim().replace(/^["']|["']$/g, "");
  const base = doc.path.split("/").pop() ?? doc.path;
  return base.endsWith(".md") ? base.slice(0, -3) : base;
}
