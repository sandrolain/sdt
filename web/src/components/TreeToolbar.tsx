import { useNavigate } from "react-router-dom";
import { MultiSelect, type UiOptionPreset, type UiOptionSection } from "./ui/MultiSelect";
import { STATE_KEYS, STATE_PRESETS, stateMeta, type StateFamily } from "../lib/statusDot";
import {
  setGroupMode,
  setHiddenStates,
  setHideEmpty,
  useTreeFilter,
  type GroupMode,
} from "../lib/treeFilterStore";
import { TreeSortControls } from "./TreeSortControls";
import { Select } from "./ui/Select";
import { TooltipButton } from "./ui/Tooltip";
import { Icon } from "../lib/icon";

interface TreeToolbarProps {
  /** When provided, a reset-layout button is rendered (dockview mode only). */
  onResetLayout?: () => void;
  /** When provided, a collapse-all button closes every tree group. */
  onCollapseAll?: () => void;
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

/** Quick family presets, derived once from the state registry. */
const STATE_PRESETS_UI: UiOptionPreset[] = STATE_PRESETS.map((preset) => ({
  id: preset.id,
  label: preset.label,
  ids: [...preset.keys],
}));

/** The three grouping modes, in increasing nesting order. */
const GROUP_MODE_OPTIONS = [
  { id: "flat", label: "Flat" },
  { id: "type", label: "By type" },
  { id: "full", label: "By type + groups" },
];

/**
 * Toolbar above the tree kind folders: sort controls, the state filter (every
 * document state the tree can show, grouped by lifecycle family), a grouping
 * toggle and an optional reset-layout action. Replaces the tab-strip header
 * actions that were hidden by the vertical edge-group tab bar.
 */
export function TreeToolbar({ onResetLayout, onCollapseAll }: TreeToolbarProps) {
  const { hiddenStates, groupMode, hideEmpty } = useTreeFilter();
  const navigate = useNavigate();
  const selected = STATE_KEYS.filter((key) => !hiddenStates.includes(key));
  return (
    <div className="tree-toolbar">
      <TreeSortControls />
      <MultiSelect
        ariaLabel="Visible states"
        label="States"
        sections={STATE_SECTIONS}
        presets={STATE_PRESETS_UI}
        selected={selected}
        onChange={(ids) => setHiddenStates(STATE_KEYS.filter((key) => !ids.includes(key)))}
        placeholder="All"
        emptyLabel="None"
        allLabel="All"
      />
      <Select
        ariaLabel="Grouping"
        label="Group"
        className="tree-grouping"
        options={GROUP_MODE_OPTIONS}
        value={groupMode}
        onChange={(key) => setGroupMode(key as GroupMode)}
      />
      <TooltipButton
        className={`doc-tab-actions__button${hideEmpty ? " is-active" : ""}`}
        label={hideEmpty ? "Show empty sections" : "Hide empty sections"}
        tooltip={hideEmpty ? "Show empty sections" : "Hide empty sections"}
        onPress={() => setHideEmpty(!hideEmpty)}
      >
        <Icon name={hideEmpty ? "visibility_off" : "visibility"} />
      </TooltipButton>
      {onCollapseAll && (
        <TooltipButton
          className="doc-tab-actions__button"
          label="Collapse all"
          tooltip="Collapse all"
          onPress={onCollapseAll}
        >
          <Icon name="unfold_less" />
        </TooltipButton>
      )}
      <TooltipButton
        className="doc-tab-actions__button"
        label="Semantic map"
        tooltip="Semantic map"
        onPress={() => navigate("/docs/map")}
      >
        <Icon name="hub" />
      </TooltipButton>
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
