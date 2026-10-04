export interface TreeStatusCounts {
  shown: number;
  total: number;
  open: number;
}

/**
 * Fixed row under the tree scroll region: how many documents the state filter
 * leaves visible out of the corpus total, and how many of those are open (the
 * effective-state `open` family the toolbar groups and presets use).
 */
export function TreeStatusBar({ shown, total, open }: TreeStatusCounts) {
  return (
    <div className="tree-status" role="status" aria-live="polite">
      <span className="tree-status__counts">
        {shown} shown · {total} total · {open} open
      </span>
    </div>
  );
}
