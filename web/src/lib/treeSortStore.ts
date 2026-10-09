import { createStore } from "./store";
import { TREE_SORTS, type TreeSortKey } from "./treeSort";
import { loadTreeView, saveTreeView } from "./treeViewStore";

/** Tree sort selection, shared between the list and the tab-bar controls. */
export interface TreeSortState {
  key: TreeSortKey;
}

const DEFAULT: TreeSortState = { key: "created_desc" };

/** The persisted sort key, when it is still one the tree offers. */
function storedKey(): TreeSortKey {
  const stored = loadTreeView().sortKey;
  return TREE_SORTS.some((sort) => sort.id === stored) ? (stored as TreeSortKey) : DEFAULT.key;
}

const store = createStore<TreeSortState>({
  initial: () => ({ key: storedKey() }),
  save: (state) => saveTreeView({ sortKey: state.key }),
  equal: (a, b) => a.key === b.key,
});

export function setTreeSortKey(key: TreeSortKey): void {
  store.set({ key });
}

export function resetTreeSort(): void {
  store.reset({ ...DEFAULT });
  saveTreeView({ sortKey: undefined });
}

export function useTreeSort(): TreeSortState {
  return store.use();
}
