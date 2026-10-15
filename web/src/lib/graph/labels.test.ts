import { describe, expect, it } from "vitest";
import {
  LABEL_CHAR_WIDTH,
  LABEL_LINE_HEIGHT,
  LABEL_MAX_LINES,
  LABEL_MAX_WIDTH,
  labelBox,
} from "./labels";

describe("labelBox", () => {
  it("keeps a short title on one line", () => {
    const box = labelBox("Auth middleware");
    expect(box.lines).toEqual(["Auth middleware"]);
    expect(box.height).toBe(LABEL_LINE_HEIGHT);
    expect(box.truncated).toBe(false);
    expect(box.width).toBeCloseTo("Auth middleware".length * LABEL_CHAR_WIDTH + 10, 5);
  });

  it("wraps a long title onto a second line instead of widening forever", () => {
    const box = labelBox(
      "The wave five semantic unification plan and its residual work in the viewer",
    );
    expect(box.lines.length).toBe(2);
    expect(box.height).toBe(2 * LABEL_LINE_HEIGHT);
    expect(box.width).toBeLessThanOrEqual(LABEL_MAX_WIDTH);
    expect(box.truncated).toBe(true);
    expect(box.lines[1].endsWith("…")).toBe(true);
    // Both rendered lines fit the width the box reserved, so the collision pass and
    // the text on screen cannot disagree.
    for (const line of box.lines) {
      expect(line.length * LABEL_CHAR_WIDTH + 10).toBeLessThanOrEqual(LABEL_MAX_WIDTH + 1);
    }
  });

  it("never renders more than the bound, however long the title", () => {
    const box = labelBox("x".repeat(500));
    expect(box.lines.length).toBe(LABEL_MAX_LINES);
    expect(box.height).toBe(LABEL_MAX_LINES * LABEL_LINE_HEIGHT);
    expect(box.width).toBeLessThanOrEqual(LABEL_MAX_WIDTH);
    expect(box.truncated).toBe(true);
  });

  it("hard-splits one very long word, which no space would ever wrap", () => {
    const box = labelBox("context/refs/a-very-long-directory-name/another/deep/path/to/a/file.md");
    expect(box.lines.length).toBe(2);
    expect(box.truncated).toBe(true);
    for (const line of box.lines) {
      expect(line.length * LABEL_CHAR_WIDTH + 10).toBeLessThanOrEqual(LABEL_MAX_WIDTH + 1);
    }
  });

  it("answers with a total box for empty and whitespace-only text", () => {
    for (const text of ["", "   "]) {
      const box = labelBox(text);
      expect(box.lines).toEqual([""]);
      expect(box.height).toBe(LABEL_LINE_HEIGHT);
      expect(box.truncated).toBe(false);
    }
  });

  it("is deterministic: the same title yields the same box", () => {
    const title = "Instruction corpus coherence and declaration to implementation correspondence";
    const first = labelBox(title);
    for (let i = 0; i < 5; i += 1) expect(labelBox(title)).toEqual(first);
  });
});
