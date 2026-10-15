import { kindLabel, type EntryFilterKind } from "./kinds";

/**
 * The kind filter's selection.
 *
 * An **empty set means every kind**, which is the `All` entry's semantic: a reader
 * who clears the selection sees the whole corpus rather than nothing, and a view that
 * mounts the filter starts on `All` without a second state for it.
 */
export type KindSelection = ReadonlySet<string>;

/** Toggle one kind in the selection, returning a new set. */
export function toggleKind(selection: KindSelection, kind: string): Set<string> {
  const next = new Set(selection);
  if (next.has(kind)) next.delete(kind);
  else next.add(kind);
  return next;
}

/** Clear the selection, which means "all kinds". */
export function clearKinds(): Set<string> {
  return new Set();
}

/** True when an entry of `kind` passes the selection. */
export function kindSelectionMatches(selection: KindSelection, kind: string): boolean {
  return selection.size === 0 || selection.has(kind);
}

/** How many kinds the trigger reports; zero is "all". */
export function kindSelectionCount(selection: KindSelection): number {
  return selection.size;
}

/** The trigger's label: `All kinds`, the single kind's name, or the count. */
export function kindSelectionLabel(selection: KindSelection): string {
  const count = kindSelectionCount(selection);
  if (count === 0) return "All kinds";
  if (count === 1) {
    const [only] = [...selection];
    return only ? kindLabel(only as EntryFilterKind) : "All kinds";
  }
  return `${count} kinds`;
}
