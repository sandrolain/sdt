/**
 * composerHistory — the walk back through the prompts already sent.
 *
 * The history is not a store of its own: the bridge already writes every prompt onto the
 * thread before the turn starts (`openUserMessage`), and the panel renders that thread, so
 * the texts are derived where they are read. This module only owns the *walk* — where the
 * reader is in the list and the draft they were writing — which is the part worth testing
 * without a DOM.
 *
 * The shape is a shell's: ArrowUp goes back, ArrowDown comes forward, and going forward
 * past the newest puts the reader back on the draft they abandoned, not on an empty field.
 */

/** One walk through the sent prompts. */
export interface ComposerWalk {
  /** Position in the sent prompts, or -1 when the reader is not walking. */
  index: number;
  /** The draft that was being written when the walk started, restored on the way out. */
  stash: string;
  /** How many prompts existed when the walk started: a new turn ends the walk. */
  count: number;
}

/** NO_WALK is the idle state: the composer holds the reader's own draft. */
export const NO_WALK: ComposerWalk = { index: -1, stash: "", count: 0 };

/** -1 walks back, 1 walks forward. */
export type WalkDirection = -1 | 1;

/**
 * caretAtEdge reports whether the caret is on the line the direction needs: the first line
 * to walk back, the last to walk forward. Anywhere else the browser's line movement wins,
 * because a multi-line prompt is edited with the arrow keys.
 */
export function caretAtEdge(text: string, caret: number, direction: WalkDirection): boolean {
  return direction === -1
    ? text.lastIndexOf("\n", caret - 1) === -1
    : text.indexOf("\n", caret) === -1;
}

/**
 * recall returns the draft and walk a key press produces, or null when the key is not the
 * composer's to take (nothing sent yet, or already at the oldest prompt).
 *
 * `draft` is the field's current text: it becomes the stash when the walk starts.
 */
export function recall(
  texts: string[],
  walk: ComposerWalk,
  direction: WalkDirection,
  draft: string,
): { walk: ComposerWalk; draft: string } | null {
  if (texts.length === 0) return null;
  // A new prompt while walking ends the old walk: the list the index pointed into is gone.
  const start =
    walk.index >= 0 && walk.count === texts.length
      ? walk
      : { index: -1, stash: draft, count: texts.length };

  if (direction === -1) {
    if (start.index === -1) {
      const index = texts.length - 1;
      return { walk: { ...start, index }, draft: texts[index] };
    }
    if (start.index === 0) return null;
    const index = start.index - 1;
    return { walk: { ...start, index }, draft: texts[index] };
  }

  if (start.index === -1) return null;
  if (start.index >= texts.length - 1) {
    return { walk: { ...start, index: -1 }, draft: start.stash };
  }
  const index = start.index + 1;
  return { walk: { ...start, index }, draft: texts[index] };
}
