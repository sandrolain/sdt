import { describe, expect, it } from "vitest";
import { SLIDES_THEME_CSS, SLIDES_THEME_NAME, slidesThemeCss } from "./slidesTheme";

describe("slidesTheme", () => {
  it("declares a Marp @theme named sdt", () => {
    expect(SLIDES_THEME_CSS).toContain(`@theme ${SLIDES_THEME_NAME}`);
    expect(SLIDES_THEME_NAME).toBe("sdt");
  });

  it("uses design tokens, never a raw hex colour", () => {
    expect(SLIDES_THEME_CSS).toContain("var(--bg-base)");
    expect(SLIDES_THEME_CSS).toContain("var(--fg)");
    expect(SLIDES_THEME_CSS).toContain("var(--accent)");
    // no literal colour anywhere in the theme
    expect(SLIDES_THEME_CSS).not.toMatch(/#[0-9a-fA-F]{3,8}\b/);
    expect(SLIDES_THEME_CSS).not.toMatch(/\brgba?\(/);
    expect(SLIDES_THEME_CSS).not.toMatch(/\bhsla?\(/);
  });

  it("returns the same stylesheet from the accessor", () => {
    expect(slidesThemeCss()).toBe(SLIDES_THEME_CSS);
  });
});
