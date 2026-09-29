import { MultiSelect, type UiOptionSection } from "./ui/MultiSelect";
import { STATE_KEYS, stateMeta, type StateFamily } from "../lib/statusDot";
import { setHiddenStates, toggleGrouped, useTreeFilter } from "../lib/treeFilterStore";
import { TreeSortControls } from "./TreeSortControls";
import { Switch } from "./ui/Switch";
import { TooltipButton } from "./ui/Tooltip";
import { Icon } from "../lib/icon";

interface TreeToolbarProps {
  /** When provided, a reset-layout button is rendered (dockview mode only). */
  onResetLayout?: () => void;
}

/** Lifecycle families, in the order the options are grouped. */
const FAMILY_ORDER: StateFamily[] = ["open", "concluded", "deferred", "retired", "unclassified"];

const FAMILY_LABELS: Record<StateFamily, string> = {
  open: "Open",
  concluded: "Concluded",
  deferred: "Deferred",
  retired: "Retired",
  unclassified: "Unclassified",
};

/** Every state the model can emit, grouped by lifecycle family, in the order
 *  `STATE_KEYS` declares them. */
const STATE_SECTIONS: UiOptionSection[] = FAMILY_ORDER.map((family) => ({
  id: family,
  label: FAMILY_LABELS[family],
  options: STATE_KEYS.filter((key) => stateMeta[key].family === family).map((key) => ({
    id: key,
    label: stateMeta[key].label,
  })),
})).filter((section) => section.options.length > 0);

/**
 * Toolbar above the tree kind folders: sort controls, the state filter (every
 * document state the tree can show, grouped by lifecycle family), a grouping
 * toggle and an optional reset-layout action. Replaces the tab-strip header
 * actions that were hidden by the vertical edge-group tab bar.
 */
export function TreeToolbar({ onResetLayout }: TreeToolbarProps) {
  const { hiddenStates, grouped } = useTreeFilter();
  const selected = STATE_KEYS.filter((key) => !hiddenStates.includes(key));
  return (
    <div className="tree-toolbar">
      <TreeSortControls />
      <MultiSelect
        ariaLabel="Visible states"
        sections={STATE_SECTIONS}
        selected={selected}
        onChange={(ids) => setHiddenStates(STATE_KEYS.filter((key) => !ids.includes(key)))}
        placeholder="All"
        emptyLabel="None"
        allLabel="All"
      />
      <Switch isSelected={grouped} onChange={toggleGrouped}>
        Grouped
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
