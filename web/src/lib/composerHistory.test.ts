import { describe, expect, it } from "vitest";
import { NO_WALK, caretAtEdge, recall, type ComposerWalk } from "./composerHistory";

const SENT = ["primo prompt", "secondo prompt"];

/** walkTo applies one movement and returns the state it left, fail-fast on an unexpected null. */
function walkTo(texts: string[], walk: ComposerWalk, direction: -1 | 1, draft: string) {
  const next = recall(texts, walk, direction, draft);
  if (!next) throw new Error(`recall returned null for direction ${direction}`);
  return next;
}

describe("caretAtEdge", () => {
  it("lets the walk back start only from the first line", () => {
    expect(caretAtEdge("due righe\nqui", 4, -1)).toBe(true);
    expect(caretAtEdge("due righe\nqui", 12, -1)).toBe(false);
  });

  it("lets the walk forward start only from the last line", () => {
    expect(caretAtEdge("due righe\nqui", 12, 1)).toBe(true);
    expect(caretAtEdge("due righe\nqui", 4, 1)).toBe(false);
  });

  it("is a single line when there is no newline at all", () => {
    expect(caretAtEdge("una riga", 0, -1)).toBe(true);
    expect(caretAtEdge("una riga", 8, 1)).toBe(true);
  });
});

describe("recall", () => {
  it("does nothing before anything was sent", () => {
    expect(recall([], NO_WALK, -1, "bozza")).toBeNull();
    expect(recall([], NO_WALK, 1, "bozza")).toBeNull();
  });

  it("walks back from the newest prompt and keeps the draft", () => {
    const first = walkTo(SENT, NO_WALK, -1, "bozza in corso");
    expect(first.draft).toBe("secondo prompt");
    expect(first.walk).toEqual({ index: 1, stash: "bozza in corso", count: 2 });

    const second = walkTo(SENT, first.walk, -1, first.draft);
    expect(second.draft).toBe("primo prompt");
    expect(second.walk.index).toBe(0);
  });

  it("stops at the oldest prompt instead of wrapping", () => {
    const oldest: ComposerWalk = { index: 0, stash: "bozza", count: 2 };
    expect(recall(SENT, oldest, -1, "primo prompt")).toBeNull();
  });

  it("walks forward and restores the draft past the newest", () => {
    const back: ComposerWalk = { index: 0, stash: "bozza in corso", count: 2 };
    const forward = walkTo(SENT, back, 1, "primo prompt");
    expect(forward.draft).toBe("secondo prompt");
    expect(forward.walk.index).toBe(1);

    const restored = walkTo(SENT, forward.walk, 1, forward.draft);
    expect(restored.draft).toBe("bozza in corso");
    expect(restored.walk).toEqual({ index: -1, stash: "bozza in corso", count: 2 });
  });

  it("does not walk forward from the reader's own draft", () => {
    expect(recall(SENT, NO_WALK, 1, "bozza")).toBeNull();
  });

  it("starts a new walk when a turn was added while the reader was looking back", () => {
    const stale: ComposerWalk = { index: 0, stash: "vecchia bozza", count: 2 };
    const next = walkTo([...SENT, "terzo prompt"], stale, -1, "bozza attuale");
    expect(next.draft).toBe("terzo prompt");
    expect(next.walk).toEqual({ index: 2, stash: "bozza attuale", count: 3 });
  });
});
