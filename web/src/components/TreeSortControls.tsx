import { Icon } from "../lib/icon";
import { TREE_SORTS, type TreeSortKey } from "../lib/treeSort";
import { setTreeSortKey, toggleTreeSortDir, useTreeSort } from "../lib/treeSortStore";
import { Select } from "./ui/Select";

/**
 * Sort controls for the documents tree: a compact key select plus a direction
 * toggle. Rendered in the tree group's tab-bar actions (and in the test
 * fallback), so the tree panel itself stays list-only.
 */
export function TreeSortControls() {
  const { key, dir } = useTreeSort();
  return (
    <div className="tree-sort-controls">
      <Select
        ariaLabel="Sort entries by"
        className="ui-select--compact"
        options={TREE_SORTS.map((s) => ({ id: s.id, label: s.label }))}
        selectedKey={key}
        onSelectionChange={(next) => setTreeSortKey(String(next) as TreeSortKey)}
      />
      <button
        type="button"
        className="tree-sort-dir"
        aria-label={dir === "asc" ? "Sort ascending" : "Sort descending"}
        title={dir === "asc" ? "Ascending" : "Descending"}
        onClick={toggleTreeSortDir}
      >
        <Icon name={dir === "asc" ? "arrow_upward" : "arrow_downward"} />
      </button>
    </div>
  );
}
