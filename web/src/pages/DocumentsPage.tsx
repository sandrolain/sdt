import { useMemo } from "react";
import { useParams } from "react-router-dom";
import { Tree } from "../components/Tree";
import { DocDetail } from "../components/DocDetail";
import { DocMetaPanel } from "../components/DocMetaPanel";
import { DockLayout, type DockPanelDef } from "../components/DockLayout";
import { useDoc } from "../lib/useDoc";

export function DocumentsPage() {
  return <DocWorkspace />;
}

/** Dockview workspace: tree | document | metadata, one doc fetch shared. */
function DocWorkspace() {
  const path = useParams()["*"];
  const { doc, error, loading } = useDoc(path ?? "");

  const panels = useMemo<DockPanelDef[]>(
    () => [
      {
        id: "tree",
        title: "Tree",
        icon: "account_tree",
        minWidth: 170,
        maxWidth: 460,
        render: () => <Tree />,
      },
      {
        id: "content",
        title: "Document",
        icon: "description",
        minWidth: 320,
        render: () => (
          <main className="content">
            {path ? (
              <DocDetail key={path} path={path} doc={doc} error={error} loading={loading} />
            ) : (
              <p className="content__empty">Select a document from the tree.</p>
            )}
          </main>
        ),
      },
      {
        id: "meta",
        title: "Metadata",
        icon: "info",
        minWidth: 190,
        maxWidth: 460,
        render: () => (path ? <DocMetaPanel doc={doc} /> : null),
      },
    ],
    [path, doc, error, loading],
  );

  return (
    <div className="workspace">
      <DockLayout storageKey="workspace" panels={panels} />
    </div>
  );
}
