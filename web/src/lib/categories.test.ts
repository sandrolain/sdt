import { describe, expect, it } from "vitest";
import { categoryColor, categoryIcon } from "./categories";

describe("categoryIcon", () => {
  it("maps every register slug from categories.yaml", () => {
    for (const slug of [
      "bug",
      "issue",
      "new-feature",
      "feature-change",
      "refactor",
      "improvement",
      "research",
    ]) {
      expect(categoryIcon(slug)).toBeTruthy();
      expect(categoryColor(slug)).toBeTruthy();
    }
  });

  it("falls back for an unknown or absent slug", () => {
    expect(categoryIcon("hotfix")).toBe("sell");
    expect(categoryColor("hotfix")).toBe("var(--ctp-overlay1)");
    expect(categoryIcon("")).toBe("sell");
  });

  it("gives distinct glyphs to the register slugs", () => {
    const icons = [
      "bug",
      "issue",
      "new-feature",
      "feature-change",
      "refactor",
      "improvement",
      "research",
    ].map(categoryIcon);
    expect(new Set(icons).size).toBe(icons.length);
  });
});
