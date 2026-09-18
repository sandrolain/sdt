import { Switch } from "./ui/Switch";
import { TreeSortControls } from "./TreeSortControls";
import { TooltipButton } from "./ui/Tooltip";
import { Icon } from "../lib/icon";
import { toggleHideCompleted, useTreeFilter } from "../lib/treeFilterStore";

interface TreeToolbarProps {
  /** When provided, a reset-layout button is rendered (dockview mode only). */
  onResetLayout?: () => void;
}

/**
 * Toolbar above the tree kind folders: sort controls plus a "not completed"
 * filter toggle, and an optional reset-layout action. Replaces the tab-strip
 * header actions that were hidden by the vertical edge-group tab bar.
 */
export function TreeToolbar({ onResetLayout }: TreeToolbarProps) {
  const { hideCompleted } = useTreeFilter();
  return (
    <div className="tree-toolbar">
      <TreeSortControls />
      <Switch isSelected={hideCompleted} onChange={toggleHideCompleted}>
        Not completed
      </Switch>
      {onResetLayout && (
        <TooltipButton
          className="doc-tab-actions__button"
          label="Reset layout"
          tooltip="Reset layout"
          onPress={onResetLayout}
        >
          <Icon name="restart_alt" />
        </TooltipButton>
      )}
    </div>
  );
}
