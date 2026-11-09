import { describe, expect, it } from "vitest";
import { normalizeDeckSource } from "./slidesSource";

describe("normalizeDeckSource", () => {
  it("drops a leading material header before the deck frontmatter", () => {
    const source =
      "<!-- material: concept=wiki/x; claims=c-1@abc -->\n\n---\ntheme: default\npaginate: true\n---\n\n# One\n";
    expect(normalizeDeckSource(source)).toBe("---\ntheme: default\npaginate: true\n---\n\n# One\n");
  });

  it("leaves a leading speaker note before a separator alone", () => {
    const source = "<!-- a note -->\n\n# One\n\n---\n\n# Two\n";
    expect(normalizeDeckSource(source)).toBe(source);
  });

  it("does not mistake a separator for frontmatter", () => {
    // `---` followed by a blank line is a slide separator, not a frontmatter
    // block; the leading note must stay (it is slide 1's note).
    const source = "<!-- a note -->\n\n---\n\n# One\n";
    expect(normalizeDeckSource(source)).toBe(source);
  });

  it("leaves corpus frontmatter at the start alone", () => {
    const source = "---\nkind: notes\n---\n\n<!-- a note -->\n\n# One\n";
    expect(normalizeDeckSource(source)).toBe(source);
  });
});
