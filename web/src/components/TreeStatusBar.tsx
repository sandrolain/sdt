import { Icon } from "../lib/icon";
import { kindColor, kindIcon, kindLabel, type EntryFilterKind } from "../lib/kinds";

export interface TreeKindCount {
  kind: EntryFilterKind;
  count: number;
}

/**
 * Fixed row under the tree scroll region: how many documents the state filter
 * leaves visible out of the corpus total, plus the per-kind breakdown of the
 * visible set (the same set the group counts describe).
 */
export function TreeStatusBar({
  shown,
  total,
  kinds,
}: {
  shown: number;
  total: number;
  kinds: TreeKindCount[];
}) {
  return (
    <div className="tree-status" role="status" aria-live="polite">
      <span className="tree-status__counts">
        {shown} shown · {total} total
      </span>
      {kinds.length > 0 && (
        <span className="tree-status__kinds">
          {kinds.map(({ kind, count }) => (
            <span key={kind} className="tree-status__kind" title={kindLabel(kind)}>
              <Icon
                name={kindIcon(kind)}
                className="tree-status__icon"
                style={{ color: kindColor(kind) }}
                label={kindLabel(kind)}
              />
              <span className="tree-status__num">{count}</span>
            </span>
          ))}
        </span>
      )}
    </div>
  );
}
