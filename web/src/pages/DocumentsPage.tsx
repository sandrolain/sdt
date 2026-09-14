import { useParams } from "react-router-dom";
import { Tree } from "../components/Tree";
import { DocDetail } from "../components/DocDetail";

export function DocumentsPage() {
  const params = useParams();
  const path = params["*"];

  return (
    <div className="workspace">
      <Tree />
      <main className="content">
        {path ? (
          <DocDetail key={path} path={path} />
        ) : (
          <p className="content__empty">Select a document from the tree.</p>
        )}
      </main>
      <aside className="panel panel--related" aria-hidden="true">
        <div className="panel-header">
          <span className="panel-header__title">Related</span>
        </div>
        <p className="content__empty">placeholder</p>
      </aside>
    </div>
  );
}
