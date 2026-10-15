import { describe, expect, it } from "vitest";
import { clearKinds, kindSelectionCount, kindSelectionMatches, toggleKind } from "./kindFilter";

describe("kind selection", () => {
  it("starts empty, which means every kind", () => {
    const selection = clearKinds();
    expect(kindSelectionCount(selection)).toBe(0);
    for (const kind of ["notes", "analysis", "decision"]) {
      expect(kindSelectionMatches(selection, kind)).toBe(true);
    }
  });

  it("toggles one kind in and out without touching the others", () => {
    let selection = clearKinds();
    selection = toggleKind(selection, "notes");
    selection = toggleKind(selection, "decision");
    expect([...selection].sort()).toEqual(["decision", "notes"]);
    expect(kindSelectionMatches(selection, "notes")).toBe(true);
    expect(kindSelectionMatches(selection, "analysis")).toBe(false);

    selection = toggleKind(selection, "notes");
    expect([...selection]).toEqual(["decision"]);
  });

  it("returns a new set and leaves the old one alone", () => {
    const before = clearKinds();
    const after = toggleKind(before, "notes");
    expect([...before]).toEqual([]);
    expect([...after]).toEqual(["notes"]);
  });

  it("clearing restores every kind rather than none", () => {
    const selection = clearKinds();
    expect(kindSelectionMatches(selection, "wiki")).toBe(true);
    expect(kindSelectionCount(selection)).toBe(0);
  });
});
