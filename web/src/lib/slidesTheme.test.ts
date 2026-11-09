import { describe, expect, it } from "vitest";
import {
  SLIDES_DEFAULT_THEME_CSS,
  SLIDES_THEME_CSS,
  SLIDES_THEME_NAME,
  slidesDefaultThemeCss,
  slidesThemeCss,
} from "./slidesTheme";

describe("slidesTheme", () => {
  it("declares a Marp @theme named sdt", () => {
    expect(SLIDES_THEME_CSS).toContain(`@theme ${SLIDES_THEME_NAME}`);
    expect(SLIDES_THEME_NAME).toBe("sdt");
  });

  it("registers the same body under the built-in default name", () => {
    expect(SLIDES_DEFAULT_THEME_CSS).toContain("@theme default");
    const body = (css: string) => css.slice(css.indexOf("\n") + 1);
    expect(body(SLIDES_DEFAULT_THEME_CSS)).toBe(body(SLIDES_THEME_CSS));
    expect(slidesDefaultThemeCss()).toBe(SLIDES_DEFAULT_THEME_CSS);
  });

  it("uses design tokens, never a raw hex colour", () => {
    for (const css of [SLIDES_THEME_CSS, SLIDES_DEFAULT_THEME_CSS]) {
      expect(css).toContain("var(--bg-base)");
      expect(css).toContain("var(--fg)");
      expect(css).toContain("var(--accent)");
      // no literal colour anywhere in the theme
      expect(css).not.toMatch(/#[0-9a-fA-F]{3,8}\b/);
      expect(css).not.toMatch(/\brgba?\(/);
      expect(css).not.toMatch(/\bhsla?\(/);
    }
  });

  it("returns the same stylesheet from the accessor", () => {
    expect(slidesThemeCss()).toBe(SLIDES_THEME_CSS);
  });
});
