import { useMemo } from "react";
import { useParams } from "react-router-dom";
import { Tree } from "../components/Tree";
import { DocMetaPanel } from "../components/DocMetaPanel";
import { DockLayout, type DockPanelDef } from "../components/DockLayout";
import { OpenDocsRail } from "../components/OpenDocsRail";
import { useDoc } from "../lib/useDoc";

export function DocumentsPage() {
  return <DocWorkspace />;
}

/** Dockview workspace: tree | open documents rail | metadata. */
function DocWorkspace() {
  const path = useParams()["*"];
  const { doc } = useDoc(path ?? "");

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
            <OpenDocsRail path={path} />
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
    [path, doc],
  );

  return (
    <div className="workspace">
      <DockLayout storageKey="workspace" panels={panels} />
    </div>
  );
}

