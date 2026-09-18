import { TREE_SORTS, type TreeSortKey } from "../lib/treeSort";
import { setTreeSortKey, useTreeSort } from "../lib/treeSortStore";
import { Select } from "./ui/Select";

/**
 * Sort controls for the documents tree: a compact key select plus a direction
 * toggle. Rendered in the tree group's tab-bar actions (and in the test
 * fallback), so the tree panel itself stays list-only.
 */
export function TreeSortControls() {
  const { key } = useTreeSort();
  return (
    <div className="tree-sort-controls">
      <Select
        ariaLabel="Sort entries by"
        className="ui-select--compact"
        options={TREE_SORTS.map((s) => ({ id: s.id, label: s.label }))}
        selectedKey={key}
        onSelectionChange={(next) => setTreeSortKey(String(next) as TreeSortKey)}
      />
    </div>
  );
}
