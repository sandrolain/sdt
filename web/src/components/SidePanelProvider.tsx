import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { SidePanelContext, type SidePanelApi, type SidePanelState } from "../lib/sidePanelContext";
import type { EdgePosition } from "../lib/workspaceLayout";

const POSITIONS: EdgePosition[] = ["left", "right"];

/**
 * Bridges the dockview edge-group collapse state to the panel-header toggles:
 * exposes `{ collapsed, toggle }` read from the api and refreshed on a collapse
 * or layout change. Renders children unconditionally; without this provider the
 * toggle reads a null context and renders nothing (the test fallback layout).
 */
export function SidePanelProvider({
  api,
  children,
}: {
  api: SidePanelApi | null;
  children: ReactNode;
}) {
  const [collapsed, setCollapsed] = useState<Record<EdgePosition, boolean>>({
    left: false,
    right: false,
  });

  useEffect(() => {
    if (!api) return;
    const sync = () => {
      setCollapsed({
        left: api.getEdgeGroup("left")?.isCollapsed() ?? false,
        right: api.getEdgeGroup("right")?.isCollapsed() ?? false,
      });
    };
    sync();
    const disposables = [
      ...POSITIONS.map((position) => api.getEdgeGroup(position)?.onDidCollapsedChange(sync)),
      api.onDidLayoutChange(sync),
    ].filter((d): d is { dispose(): void } => d !== undefined);
    return () => {
      for (const d of disposables) d.dispose();
    };
  }, [api]);

  const toggle = useCallback(
    (position: EdgePosition) => {
      const group = api?.getEdgeGroup(position);
      if (!group) return;
      if (group.isCollapsed()) group.expand();
      else group.collapse();
    },
    [api],
  );

  const value = useMemo<SidePanelState>(() => ({ collapsed, toggle }), [collapsed, toggle]);

  return <SidePanelContext.Provider value={value}>{children}</SidePanelContext.Provider>;
}
