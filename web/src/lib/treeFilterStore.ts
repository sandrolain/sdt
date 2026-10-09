import { createStore } from "./store";
import { loadTreeView, saveTreeView } from "./treeViewStore";

/** How the tree nests its entries. `flat` renders every document in one list
 *  with no kind folders; `type` groups by document kind with flat contents;
 *  `full` also nests the objective/plan sub-folders inside a kind folder. */
export type GroupMode = "flat" | "type" | "full";

/** Every grouping mode, in toolbar order — the single list the value validates against. */
export const GROUP_MODES: GroupMode[] = ["flat", "type", "full"];

/** Tree filter selection: the document states the tree shows, and how the tree
 *  groups its entries. The selection is stored as its complement (the hidden
 *  states), so the default — nothing hidden — is today's default tree and no
 *  selection can empty the tree by accident. */
export interface TreeFilterState {
  hiddenStates: string[];
  groupMode: GroupMode;
  /** Hide kind folders with no visible document (a decluttered view). */
  hideEmpty: boolean;
}

const DEFAULT: TreeFilterState = { hiddenStates: [], groupMode: "type", hideEmpty: false };

/** The persisted slices, each dropped when it no longer has the right shape. */
function storedState(): TreeFilterState {
  const stored = loadTreeView();
  const hidden = stored.hiddenStates;
  const mode = stored.groupMode;
  return {
    hiddenStates: Array.isArray(hidden)
      ? hidden.filter((s): s is string => typeof s === "string")
      : DEFAULT.hiddenStates,
    groupMode: GROUP_MODES.includes(mode as GroupMode) ? (mode as GroupMode) : DEFAULT.groupMode,
    hideEmpty: typeof stored.hideEmpty === "boolean" ? stored.hideEmpty : DEFAULT.hideEmpty,
  };
}

const store = createStore<TreeFilterState>({ initial: storedState });

/** Replace the set of hidden states; the caller passes the complement of what
 *  the control shows selected. */
export function setHiddenStates(states: string[]): void {
  store.set({ ...store.get(), hiddenStates: states });
  saveTreeView({ hiddenStates: states });
}

/** Replace the grouping mode. The three modes are mutually exclusive, so one
 *  closed value cannot enter an invalid state. */
export function setGroupMode(mode: GroupMode): void {
  store.set({ ...store.get(), groupMode: mode });
  saveTreeView({ groupMode: mode });
}

/** Toggle the hide-empty-sections flag. */
export function setHideEmpty(hideEmpty: boolean): void {
  store.set({ ...store.get(), hideEmpty });
  saveTreeView({ hideEmpty });
}

export function resetTreeFilter(): void {
  store.reset({ ...DEFAULT });
  saveTreeView({ hiddenStates: undefined, groupMode: undefined, hideEmpty: undefined });
}

export function useTreeFilter(): TreeFilterState {
  return store.use();
}
