import { useCallback, useRef, type RefObject } from "react";
import type { DockviewApi, DockviewReadyEvent } from "dockview-react";
import { clearLayout, loadLayout, saveLayout } from "./layoutStore";

/** Dockview needs real layout measurement; tests use a plain columns fallback. */
export const DOCKVIEW_ENABLED = import.meta.env.MODE !== "test";

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
