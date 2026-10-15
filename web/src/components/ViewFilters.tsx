/**
 * ViewFilters — the status filter Gallery, Timeline and Kanban mount.
 *
 * It is the Documents tree's own control and vocabulary (`STATE_PRESETS` in
 * `lib/statusDot.ts`), so a reader meets one affordance and one set of words
 * wherever they filter by state.
 */
import { Select } from "./ui/Select";
import { statePresetOptions } from "../lib/viewFilter";
import type { StatePresetId } from "../lib/statusDot";

export function ViewFilters({
  preset,
  onChange,
  label = "Status",
}: {
  preset: StatePresetId;
  onChange: (id: StatePresetId) => void;
  label?: string;
}) {
  return (
    <label className="view-filters">
      <span className="view-filters__label">{label}</span>
      <Select
        ariaLabel="Filter by status"
        compact
        options={statePresetOptions()}
        selectedKey={preset}
        onSelectionChange={(key) => onChange(String(key) as StatePresetId)}
      />
    </label>
  );
}
