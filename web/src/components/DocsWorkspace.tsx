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
import {
  clearLayout,
  loadLayout,
  resetLayout,
  saveLayout,
  WORKSPACE_STORAGE_KEY,
} from "../lib/layoutStore";
import {
  addSidePanels,
  COURTESY_PANEL_ID,
  DOC_PANEL_PREFIX,
  ensureCenterGroup,
  type CenterGroup,
} from "../lib/workspaceLayout";
import { tabContextMenuItems } from "../lib/workspaceTabs";
import { displayTitle } from "../lib/titles";
import { Icon } from "../lib/icon";
import { useDoc } from "../lib/useDoc";
import { Tree } from "./Tree";
import { DocDetail } from "./DocDetail";
import { DocMetaPanel } from "./DocMetaPanel";
import { WorkspaceTab } from "./WorkspaceTab";
import { DocTabHeader } from "./DocTabHeader";
import { TooltipButton } from "./ui/Tooltip";
import { kindColor, kindFromPath, kindIcon } from "../lib/kinds";

const DOC_PREFIX = DOC_PANEL_PREFIX;
const COURTESY_ID = COURTESY_PANEL_ID;
const docPanelId = (path: string) => `${DOC_PREFIX}${path}`;

/** Dockview needs real layout measurement; tests use a plain columns fallback. */
const DOCKVIEW_ENABLED = import.meta.env.MODE !== "test";

/** Stable dockview component registry (module scope: never recreated). */
const components: Record<string, (props: IDockviewPanelProps) => ReactNode> = {
  tree: () => (
    <div className="dock-content">
      <Tree onResetLayout={() => resetLayout(WORKSPACE_STORAGE_KEY)} />
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

/** Header actions: close-all on document tabs. */
function DocHeaderActions({ group }: IDockviewHeaderActionsProps) {
  const { closeAll } = useOpenDocs();
  const hasDocs = group.panels.some((p) => p.id.startsWith(DOC_PREFIX));
  if (!hasDocs) return null;
  return (
    <div className="doc-tab-actions">
      <TooltipButton
        className="doc-tab-actions__button"
        label="Close all documents"
        tooltip="Close all documents"
        onPress={closeAll}
      >
        <Icon name="close" />
      </TooltipButton>
    </div>
  );
}

/** Documents workspace: tree | document tabs | metadata, all dockview panels. */
export function DocsWorkspace() {
  const { state, activate, close } = useOpenDocs();
  const apiRef = useRef<DockviewApi | null>(null);
  const centerRef = useRef<CenterGroup | null>(null);
  const [ready, setReady] = useState(false);

  const onReady = useCallback(
    (event: DockviewReadyEvent) => {
      const api = event.api;
      apiRef.current = api;

      const stored = loadLayout(WORKSPACE_STORAGE_KEY);
      if (stored) {
        try {
          api.fromJSON(stored as never);
        } catch {
          clearLayout(WORKSPACE_STORAGE_KEY);
        }
      }
      // always ensure the side panels exist: recovers layouts persisted by
      // older builds where tree/meta were closable
      addSidePanels(api);

      api.onDidActivePanelChange((event) => {
        const panel = event.panel;
        if (panel?.id.startsWith(DOC_PREFIX)) activate(panel.id.slice(DOC_PREFIX.length));
      });
      api.onDidRemovePanel((panel: IDockviewPanel) => {
        if (panel.id.startsWith(DOC_PREFIX)) close(panel.id.slice(DOC_PREFIX.length));
      });
      api.onDidLayoutChange(() => saveLayout(WORKSPACE_STORAGE_KEY, api.toJSON()));
      setReady(true);
    },
    [activate, close],
  );

  // state → panels: document tabs (centre group) and follow the active document
  useEffect(() => {
    const api = apiRef.current;
    if (!ready || !api) return;
    const center = ensureCenterGroup(api, centerRef);

    // add the placeholder first so the centre group never goes empty while the
    // last document is being removed (an empty grid group is torn down)
    if (state.docs.length === 0 && !api.getPanel(COURTESY_ID)) {
      api.addPanel({
        id: COURTESY_ID,
        component: "courtesy",
        title: "No documents",
        position: { referenceGroup: center.id },
        minimumWidth: 320,
      });
    }
    for (const path of state.docs) {
      if (api.getPanel(docPanelId(path))) continue;
      api.addPanel({
        id: docPanelId(path),
        component: "doc",
        tabComponent: "doc",
        title: displayTitle({ path }),
        params: { path },
        position: { referenceGroup: center.id },
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
    if (state.docs.length > 0) {
      const courtesy = api.getPanel(COURTESY_ID);
      if (courtesy) api.removePanel(courtesy);
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
      defaultTabComponent={WorkspaceTab}
      tabComponents={{ doc: DocTabHeader }}
      getTabContextMenuItems={(params) => tabContextMenuItems(params.panel.id)}
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
              {state.docs.map((path) => {
                const kind = kindFromPath(path);
                return (
                  <button
                    key={path}
                    type="button"
                    role="tab"
                    aria-selected={path === state.active}
                    className={path === state.active ? "is-active" : ""}
                    onClick={() => activate(path)}
                  >
                    <Icon
                      name={kindIcon(kind)}
                      className="dock-doc-tab__icon"
                      style={{ color: kindColor(kind) }}
                    />
                    <span className="dock-doc-tab__label">{displayTitle({ path })}</span>
                  </button>
                );
              })}
              <TooltipButton
                className="doc-tab-actions__button"
                label="Close all documents"
                tooltip="Close all documents"
                onPress={closeAll}
              >
                <Icon name="close" />
              </TooltipButton>
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
