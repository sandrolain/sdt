import type { IDockviewPanelHeaderProps } from "dockview-react";
import { Icon } from "../lib/icon";
import { useSidePanelOptional } from "../lib/sidePanelContext";
import { SIDE_PANELS } from "../lib/workspaceLayout";

/**
 * Edge-panel tab: the panel icon that replaces the default title tab. It stays
 * visible when the group collapses (the rail) and its click toggles the panel,
 * so a hidden side panel is always recoverable.
 */
export function SideRailTab(props: IDockviewPanelHeaderProps) {
  const side = useSidePanelOptional();
  const spec = SIDE_PANELS.find((s) => s.id === props.api.id);
  if (!side || !spec) return null;
  const collapsed = side.collapsed[spec.position];
  const title = `${collapsed ? "Show" : "Hide"} ${spec.id === "tree" ? "tree" : "panel"}`;
  return (
    <button
      type="button"
      className="side-rail-tab"
      aria-label={title}
      title={title}
      data-collapsed={collapsed || undefined}
      onClick={(event) => {
        // the wrapping dockview tab must not also toggle the collapse
        event.stopPropagation();
        side.toggle(spec.position);
      }}
    >
      <Icon name={`${spec.position}_panel_${collapsed ? "open" : "close"}`} />
    </button>
  );
}
