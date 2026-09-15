import { useParams } from "react-router-dom";
import { Tree } from "../components/Tree";
import { DocDetail } from "../components/DocDetail";
import { DocMetaPanel } from "../components/DocMetaPanel";
import { useDoc } from "../lib/useDoc";

export function DocumentsPage() {
  return (
    <div className="workspace">
      <Tree />
      <DocWorkspace />
    </div>
  );
}

/** Main document column plus the shared metadata panel, fed by one doc fetch. */
function DocWorkspace() {
  const path = useParams()["*"];
  const { doc, error, loading } = useDoc(path ?? "");

  return (
    <>
      <main className="content">
        {path ? (
          <DocDetail key={path} path={path} doc={doc} error={error} loading={loading} />
        ) : (
          <p className="content__empty">Select a document from the tree.</p>
        )}
      </main>
      {path && <DocMetaPanel doc={doc} />}
    </>
  );
}
