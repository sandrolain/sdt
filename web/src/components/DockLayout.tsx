import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FunctionComponent,
  type ReactNode,
} from "react";
import {
  GridviewReact,
  Orientation,
  type GridviewApi,
  type GridviewReadyEvent,
  type IGridviewPanelProps,
} from "dockview-react";
import { clearLayout, loadLayout, saveLayout } from "../lib/layoutStore";
import { LayoutContext } from "../lib/layoutContext";
import { Icon } from "../lib/icon";

/** Dockview needs real layout measurement; tests use a plain flex fallback. */
const DOCKVIEW_ENABLED = import.meta.env.MODE !== "test";

export interface DockPanelDef {
  id: string;
  title: string;
  icon?: string;
  minWidth?: number;
  maxWidth?: number;
  render: () => ReactNode;
}

interface DockLayoutProps {
  /** localStorage namespace, e.g. "workspace" */
  storageKey: string;
  panels: DockPanelDef[];
}

/** Resizable dockview grid with collapse toggles and persisted layout. */
export function DockLayout({ storageKey, panels }: DockLayoutProps) {
  const [hidden, setHidden] = useState<string[]>(() => {
    const stored = loadLayout(`${storageKey}:hidden`);
    return Array.isArray(stored) ? (stored as string[]) : [];
  });
  const apiRef = useRef<GridviewApi | null>(null);

  const hide = useCallback(
    (id: string) => setHidden((prev) => (prev.includes(id) ? prev : [...prev, id])),
    [],
  );
  const show = useCallback((id: string) => setHidden((prev) => prev.filter((h) => h !== id)), []);

  useEffect(() => {
    saveLayout(`${storageKey}:hidden`, hidden);
    for (const def of panels) {
      apiRef.current?.getPanel(def.id)?.api.setVisible(!hidden.includes(def.id));
    }
  }, [hidden, panels, storageKey]);

  const layoutApi = useMemo(() => ({ hidden, hide, show }), [hidden, hide, show]);

  const panelHeader = useCallback(
    (def: DockPanelDef) => (
      <header className="dock-panel__header">
        {def.icon && <Icon name={def.icon} />}
        <span className="dock-panel__title">{def.title}</span>
        <button
          type="button"
          className="dock-panel__toggle"
          title={`Hide ${def.title}`}
          onClick={() => hide(def.id)}
        >
          <Icon name="left_panel_close" />
        </button>
      </header>
    ),
    [hide],
  );

  const components = useMemo(() => {
    const map: Record<string, FunctionComponent<IGridviewPanelProps>> = {};
    for (const def of panels) {
      map[def.id] = function DockPanel() {
        return (
          <section className={`dock-panel dock-panel--${def.id}`} aria-label={def.title}>
            {panelHeader(def)}
            <div className="dock-panel__body">{def.render()}</div>
          </section>
        );
      };
    }
    return map;
  }, [panels, panelHeader]);

  const onReady = useCallback(
    (event: GridviewReadyEvent) => {
      apiRef.current = event.api;
      let restored = false;
      const stored = loadLayout(storageKey);
      if (stored) {
        try {
          event.api.fromJSON(stored as never);
          restored = event.api.panels.length > 0;
        } catch {
          clearLayout(storageKey);
        }
      }
      if (!restored) {
        for (const def of panels) {
          event.api.addPanel({
            id: def.id,
            component: def.id,
            minimumWidth: def.minWidth,
            maximumWidth: def.maxWidth,
          });
        }
      }
      for (const def of panels) {
        event.api.getPanel(def.id)?.api.setVisible(!hidden.includes(def.id));
      }
      event.api.onDidLayoutChange(() => saveLayout(storageKey, event.api.toJSON()));
    },
    [storageKey, panels, hidden],
  );

  if (!DOCKVIEW_ENABLED) {
    return (
      <LayoutContext.Provider value={layoutApi}>
        <div className="dock-layout dock-layout--fallback" data-testid="dock-layout">
          <ReopenBar panels={panels} hidden={hidden} show={show} />
          {panels.map((def) =>
            hidden.includes(def.id) ? null : (
              <section
                key={def.id}
                className={`dock-panel dock-panel--${def.id}`}
                aria-label={def.title}
              >
                {panelHeader(def)}
                <div className="dock-panel__body">{def.render()}</div>
              </section>
            ),
          )}
        </div>
      </LayoutContext.Provider>
    );
  }

  return (
    <LayoutContext.Provider value={layoutApi}>
      <div className="dock-layout">
        <ReopenBar panels={panels} hidden={hidden} show={show} />
        <GridviewReact
          className="dock-layout__grid"
          orientation={Orientation.VERTICAL}
          components={components}
          onReady={onReady}
        />
      </div>
    </LayoutContext.Provider>
  );
}

function ReopenBar({
  panels,
  hidden,
  show,
}: {
  panels: DockPanelDef[];
  hidden: string[];
  show: (id: string) => void;
}) {
  const hiddenDefs = panels.filter((def) => hidden.includes(def.id));
  if (hiddenDefs.length === 0) return null;
  return (
    <div className="dock-reopen" role="group" aria-label="Hidden panels">
      {hiddenDefs.map((def) => (
        <button
          key={def.id}
          type="button"
          className="dock-reopen__button"
          onClick={() => show(def.id)}
        >
          <Icon name="left_panel_open" />
          {def.title}
        </button>
      ))}
    </div>
  );
}
