import {
  isCanvas,
  isMermaid,
  type CanvasResponse,
  type DocResponse,
  type MermaidResponse,
} from "../lib/api";
import { displayTitle, frontmatterTitle } from "../lib/titles";
import { DocumentView } from "./DocumentView";
import { SkeletonLines } from "./Skeleton";

type AnyDoc = DocResponse | CanvasResponse | MermaidResponse;

interface DocDetailProps {
  /** corpus-relative path from the URL, e.g. context/wiki/foo.md */
  path: string;
  doc: AnyDoc | null;
  error: string | null;
  loading: boolean;
}

export function DocDetail({ path, doc, error, loading }: DocDetailProps) {
  if (error) return <p className="content__empty">doc error: {error}</p>;
  if (loading || !doc) return <SkeletonLines count={6} label={`Loading ${path}`} />;

  if (isCanvas(doc)) {
    return (
      <article>
        <header className="doc-header">
          <h1 className="doc-header__title">{displayTitle({ path: doc.path })}</h1>
        </header>
        <pre className="doc-markdown">{JSON.stringify(doc.canvas, null, 2)}</pre>
      </article>
    );
  }

  if (isMermaid(doc)) {
    return (
      <article>
        <header className="doc-header">
          <h1 className="doc-header__title">{displayTitle({ path: doc.path })}</h1>
        </header>
        <DocumentView path={doc.path} markdown={doc.source} />
      </article>
    );
  }

  return (
    <article>
      <header className="doc-header">
        <h1 className="doc-header__title">{heading(doc)}</h1>
      </header>
      <DocumentView path={doc.path} frontmatter={doc.frontmatter} markdown={doc.markdown} />
    </article>
  );
}

function heading(doc: DocResponse): string {
  return displayTitle({
    title: frontmatterTitle(doc.frontmatter),
    markdown: doc.markdown,
    path: doc.path,
  });
}
