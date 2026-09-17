import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  DockviewReact,
  type DockviewApi,
  type DockviewReadyEvent,
  type IDockviewHeaderActionsProps,
  type IDockviewPanelProps,
  type IDockviewPanel,
} from "dockview-react";
import { useOpenDocs } from "../lib/openDocsContext";
import { clearLayout, loadLayout, saveLayout } from "../lib/layoutStore";
import { displayTitle } from "../lib/titles";
import { Icon } from "../lib/icon";
import { useDoc } from "../lib/useDoc";
import { Tree } from "./Tree";
import { DocDetail } from "./DocDetail";
import { DocMetaPanel } from "./DocMetaPanel";

const STORAGE_KEY = "workspace";
const DOC_PREFIX = "doc:";
const COURTESY_ID = "doc-courtesy";
const docPanelId = (path: string) => `${DOC_PREFIX}${path}`;

/** Dockview needs real layout measurement; tests use a plain columns fallback. */
const DOCKVIEW_ENABLED = import.meta.env.MODE !== "test";

/** Stable dockview component registry (module scope: never recreated). */
const components: Record<string, (props: IDockviewPanelProps) => ReactNode> = {
  tree: () => (
    <div className="dock-content">
      <Tree />
    </div>
  ),
  doc: (props: IDockviewPanelProps) => <DocTab path={String(props.params?.["path"] ?? "")} />,
  meta: () => (
    <div className="dock-content">
      <MetaTab />
    </div>
  ),
  courtesy: () => (
    <div className="dock-content">
      <p className="content__empty">No open documents. Select one from the tree.</p>
    </div>
  ),
};

function DocTab({ path }: { path: string }) {
  const { doc, error, loading } = useDoc(path);
  return (
    <div className="dock-content doc-tab">
      <DocDetail path={path} doc={doc} error={error} loading={loading} />
    </div>
  );
}

function MetaTab() {
  const { state } = useOpenDocs();
  const path = state.active;
  const { doc } = useDoc(path ?? "");
  if (!path) return <p className="content__empty">No document selected.</p>;
  return <DocMetaPanel doc={doc} />;
}

/** Close-all action rendered at the end of the document tabs bar. */
function DocHeaderActions({ group }: IDockviewHeaderActionsProps) {
  const { closeAll } = useOpenDocs();
  const hasDocs = group.panels.some((p) => p.id.startsWith(DOC_PREFIX));
  if (!hasDocs) return null;
  return (
    <div className="doc-tab-actions">
      <button
        type="button"
        className="doc-tab-actions__button"
        title="Close all documents"
        aria-label="Close all documents"
        onClick={closeAll}
      >
        <Icon name="close_all" />
      </button>
    </div>
  );
}

/** Documents workspace: tree | document tabs | metadata, all dockview panels. */
export function DocsWorkspace() {
  const { state, activate, close } = useOpenDocs();
  const apiRef = useRef<DockviewApi | null>(null);
  const [ready, setReady] = useState(false);

  const onReady = useCallback(
    (event: DockviewReadyEvent) => {
      const api = event.api;
      apiRef.current = api;

      let restored = false;
      const stored = loadLayout(STORAGE_KEY);
      if (stored) {
        try {
          api.fromJSON(stored as never);
          restored = api.panels.length > 0;
        } catch {
          clearLayout(STORAGE_KEY);
        }
      }
      if (!restored) {
        api.addPanel({
          id: "tree",
          component: "tree",
          title: "Tree",
          initialWidth: 260,
          minimumWidth: 170,
          maximumWidth: 460,
        });
        api.addPanel({
          id: "meta",
          component: "meta",
          title: "Metadata",
          position: { direction: "right" },
          initialWidth: 320,
          minimumWidth: 190,
          maximumWidth: 560,
        });
      }

      api.onDidActivePanelChange((event) => {
        const panel = event.panel;
        if (panel?.id.startsWith(DOC_PREFIX)) activate(panel.id.slice(DOC_PREFIX.length));
      });
      api.onDidRemovePanel((panel: IDockviewPanel) => {
        if (panel.id.startsWith(DOC_PREFIX)) close(panel.id.slice(DOC_PREFIX.length));
      });
      api.onDidLayoutChange(() => saveLayout(STORAGE_KEY, api.toJSON()));
      setReady(true);
    },
    [activate, close],
  );

  // state → panels: add/remove document tabs and follow the active document
  useEffect(() => {
    const api = apiRef.current;
    if (!ready || !api) return;
    for (const path of state.docs) {
      if (api.getPanel(docPanelId(path))) continue;
      const first = state.docs[0];
      api.addPanel({
        id: docPanelId(path),
        component: "doc",
        title: displayTitle({ path }),
        params: { path },
        position:
          first && first !== path
            ? { referencePanel: docPanelId(first), direction: "within" }
            : { referencePanel: "meta", direction: "left" },
        minimumWidth: 320,
      });
    }
    for (const panel of api.panels) {
      if (
        panel.id.startsWith(DOC_PREFIX) &&
        !state.docs.includes(panel.id.slice(DOC_PREFIX.length))
      ) {
        api.removePanel(panel);
      }
    }
    // courtesy tab: shown only while no document is open
    const courtesy = api.getPanel(COURTESY_ID);
    if (state.docs.length === 0) {
      if (!courtesy) {
        api.addPanel({
          id: COURTESY_ID,
          component: "courtesy",
          title: "No documents",
          position: { referencePanel: "meta", direction: "left" },
          minimumWidth: 320,
        });
      }
    } else if (courtesy) {
      api.removePanel(courtesy);
    }
    if (state.active) {
      const panel = api.getPanel(docPanelId(state.active));
      if (panel && !panel.api.isActive) panel.api.setActive();
    }
  }, [ready, state.docs, state.active]);

  const className = useMemo(() => "docs-workspace dockview-theme-dark dock-layout", []);

  if (!DOCKVIEW_ENABLED) return <FallbackWorkspace />;

  return (
    <DockviewReact
      className={className}
      components={components}
      onReady={onReady}
      rightHeaderActionsComponent={DocHeaderActions}
    />
  );
}

/** Test-only stand-in: the same panels as a static grid (no layout measurement). */
function FallbackWorkspace() {
  const { state, activate, closeAll } = useOpenDocs();
  return (
    <div className="dock-layout dock-layout--fallback" data-testid="docs-workspace">
      <section className="dock-content" aria-label="Tree">
        <Tree />
      </section>
      <section className="dock-content doc-tab" aria-label="Document">
        {state.docs.length === 0 ? (
          <p className="content__empty">No open documents. Select one from the tree.</p>
        ) : (
          <>
            <div className="fallback-tabs" role="tablist" aria-label="Open documents">
              {state.docs.map((path) => (
                <button
                  key={path}
                  type="button"
                  role="tab"
                  aria-selected={path === state.active}
                  className={path === state.active ? "is-active" : ""}
                  onClick={() => activate(path)}
                >
                  {displayTitle({ path })}
                </button>
              ))}
              <button
                type="button"
                className="doc-tab-actions__button"
                title="Close all documents"
                aria-label="Close all documents"
                onClick={closeAll}
              >
                <Icon name="close_all" />
              </button>
            </div>
            {state.active && <DocTab path={state.active} />}
          </>
        )}
      </section>
      <section className="dock-content" aria-label="Metadata">
        <MetaTab />
      </section>
    </div>
  );
}
