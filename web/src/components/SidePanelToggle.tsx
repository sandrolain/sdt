import { Icon } from "../lib/icon";
import { useSidePanelOptional } from "../lib/sidePanelContext";
import type { EdgePosition } from "../lib/workspaceLayout";
import { TooltipButton } from "./ui/Tooltip";

/**
 * Panel-header icon that collapses/expands one side panel. Renders nothing when
 * the dockview bridge is absent (e.g. the test fallback layout), so it never
 * appears where there is no edge group to toggle.
 */
export function SidePanelToggle({ position, label }: { position: EdgePosition; label: string }) {
  const side = useSidePanelOptional();
  if (!side) return null;
  const collapsed = side.collapsed[position];
  const title = `${collapsed ? "Show" : "Hide"} ${label}`;
  return (
    <TooltipButton
      className="side-panel-toggle"
      label={title}
      tooltip={title}
      onPress={() => side.toggle(position)}
      data-collapsed={collapsed || undefined}
    >
      <Icon name={`${position}_panel_${collapsed ? "open" : "close"}`} />
    </TooltipButton>
  );
}
