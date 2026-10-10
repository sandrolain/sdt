import { useCallback, useContext, useRef, type RefObject } from "react";
import type { DockviewApi, DockviewReadyEvent } from "dockview-react";
import { themeCatppuccinMochaSpaced, themeLightSpaced } from "dockview-react";
import { clearLayout, loadLayout, saveLayout } from "./layoutStore";
import { ThemeContext } from "./theme-context";

/** Dockview needs real layout measurement; tests use a plain columns fallback. */
export const DOCKVIEW_ENABLED = import.meta.env.MODE !== "test";

/** The dockview theme object for the app's applied theme: dockview's own light
 * theme in light mode, the spaced Catppuccin Mocha theme otherwise. The app's
 * tokens still own the colours through `styles/dockview.css`. */
export function useDockviewTheme() {
  const theme = useContext(ThemeContext);
  return theme?.applied === "light" ? themeLightSpaced : themeCatppuccinMochaSpaced;
}

/** Minimal dockview API surface for placing panels in a grid. */
export interface GridPanelApi {
  getPanel(id: string): unknown;
  addPanel(options: {
    id: string;
    component: string;
    title: string;
    minimumWidth?: number;
    minimumHeight?: number;
    position?: { referencePanel: string; direction: "left" | "right" | "above" | "below" };
  }): unknown;
}

export interface GridPanelSpec {
  id: string;
  component: string;
  title: string;
  minimumWidth?: number;
  minimumHeight?: number;
}

/**
 * Place panels as a row-major grid of `columns` columns — each panel in its own
 * group (a real grid, not tabs). The first panel anchors the grid; each
 * following panel docks to the `right` of the previous one until the row is
 * full, then the next row docks `below` the first column of the previous row.
 * Idempotent: an existing panel is left in place.
 */
export function addGridPanels(api: GridPanelApi, specs: GridPanelSpec[], columns: number): void {
  const cols = Math.max(1, columns);
  specs.forEach((s, i) => {
    if (api.getPanel(s.id)) return;
    const base = {
      id: s.id,
      component: s.component,
      title: s.title,
      minimumWidth: s.minimumWidth,
      minimumHeight: s.minimumHeight,
    };
    if (i === 0) {
      api.addPanel(base);
      return;
    }
    const position =
      i % cols === 0
        ? { referencePanel: specs[i - cols].id, direction: "below" as const }
        : { referencePanel: specs[i - 1].id, direction: "right" as const };
    api.addPanel({ ...base, position });
  });
}

export interface DockviewWorkspaceHooks {
  /** Ensure the host's fixed center panel exists (idempotent). */
  ensureCenter?: (api: DockviewApi) => void;
  /** Add the host's side panels (idempotent). */
  addPanels?: (api: DockviewApi) => void;
  /** Extra host-specific wiring, run after the standard bootstrap. */
  onReady?: (api: DockviewApi) => void;
}

export interface DockviewWorkspace {
  /** False in tests: the host renders its static fallback instead. */
  enabled: boolean;
  apiRef: RefObject<DockviewApi | null>;
  onReady: (event: DockviewReadyEvent) => void;
}

/**
 * The dockview bootstrap every workspace host repeats (W3): restore the persisted
 * layout (dropping a corrupt one), ensure the fixed panels, wire layout
 * persistence — plus the test-mode fallback flag. Behaviour-preserving extraction
 * of the copy-pasted `loadLayout → fromJSON → clearLayout → addPanels →
 * onDidLayoutChange` sequence.
 */
export function useDockviewWorkspace(
  storageKey: string,
  hooks: DockviewWorkspaceHooks = {},
): DockviewWorkspace {
  const apiRef = useRef<DockviewApi | null>(null);
  // dockview calls `onReady` once, on mount (an empty-deps effect), so recreating
  // it when the host callbacks change is harmless.
  const onReady = useCallback(
    (event: DockviewReadyEvent) => {
      const api = event.api;
      apiRef.current = api;
      const stored = loadLayout(storageKey);
      if (stored) {
        try {
          api.fromJSON(stored as never);
        } catch {
          clearLayout(storageKey);
        }
      }
      hooks.ensureCenter?.(api);
      hooks.addPanels?.(api);
      hooks.onReady?.(api);
      api.onDidLayoutChange(() => saveLayout(storageKey, api.toJSON()));
    },
    [storageKey, hooks],
  );
  return { enabled: DOCKVIEW_ENABLED, apiRef, onReady };
}
