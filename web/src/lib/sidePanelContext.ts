import { createContext, useContext } from "react";
import type { EdgePosition } from "./workspaceLayout";

/**
 * Minimal dockview surface the side-panel bridge needs. Declared structurally so
 * the toggle and the provider stay testable without a real `DockviewApi` (the
 * real `DockviewGroupPanelApi` satisfies it).
 */
export interface SidePanelGroup {
  isCollapsed(): boolean;
  collapse(): void;
  expand(): void;
  onDidCollapsedChange(listener: () => void): { dispose(): void };
}

export interface SidePanelApi {
  getEdgeGroup(position: EdgePosition): SidePanelGroup | undefined;
  onDidLayoutChange(listener: () => void): { dispose(): void };
}

export interface SidePanelState {
  /** per-position collapsed flag, read from dockview */
  collapsed: Record<EdgePosition, boolean>;
  /** collapse/expand one side panel */
  toggle: (position: EdgePosition) => void;
}

export const SidePanelContext = createContext<SidePanelState | null>(null);

/** Optional read: a component in a layout without edge groups renders nothing. */
export function useSidePanelOptional(): SidePanelState | null {
  return useContext(SidePanelContext);
}
