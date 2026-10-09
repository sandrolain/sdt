/** Versioned localStorage record for the tree presentation state. */

/** Namespaced key holding the sort key, the hidden states and the group mode. */
export const TREE_VIEW_KEY = "sdt-tree-view";

/** Bumped when the stored shape changes; a mismatch drops the record. */
export const TREE_VIEW_VERSION = 1;

/**
 * The persisted slices. Each store writes and validates **only its own** slice,
 * so persistence adds no import cycle between the stores that own the values.
 */
export interface TreeViewSlices {
  sortKey?: unknown;
  hiddenStates?: unknown;
  groupMode?: unknown;
  hideEmpty?: unknown;
}

function read(): Record<string, unknown> {
  try {
    const raw = localStorage.getItem(TREE_VIEW_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as { version?: unknown };
    if (parsed?.version !== TREE_VIEW_VERSION) {
      localStorage.removeItem(TREE_VIEW_KEY);
      return {};
    }
    return parsed as unknown as Record<string, unknown>;
  } catch {
    localStorage.removeItem(TREE_VIEW_KEY);
    return {};
  }
}

/** The stored slices, or an empty record when absent, stale or corrupt. */
export function loadTreeView(): TreeViewSlices {
  const { version: _version, ...slices } = read();
  return slices;
}

/**
 * Merge one store's slice into the record. An `undefined` value clears that
 * slice, so a control reset removes exactly what it wrote; a record left with
 * nothing but its version is removed. Storage failures are ignored —
 * persistence is a convenience, never a correctness requirement.
 */
export function saveTreeView(patch: TreeViewSlices): void {
  try {
    const record: Record<string, unknown> = { version: TREE_VIEW_VERSION, ...loadTreeView() };
    for (const [key, value] of Object.entries(patch)) {
      if (value === undefined) delete record[key];
      else record[key] = value;
    }
    if (Object.keys(record).length === 1) {
      localStorage.removeItem(TREE_VIEW_KEY);
      return;
    }
    localStorage.setItem(TREE_VIEW_KEY, JSON.stringify(record));
  } catch {
    // storage full or disabled: persistence is non-essential
  }
}

/** Drop the whole record (both stores reset their own slices individually). */
export function clearTreeView(): void {
  try {
    localStorage.removeItem(TREE_VIEW_KEY);
  } catch {
    // ignore
  }
}
