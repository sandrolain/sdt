import { describe, expect, it } from "vitest";
import { FALLBACK_THEMES, PRESET_COLORS, resolveColor, themeFromTokens } from "./theme";

describe("resolveColor", () => {
  it("maps a preset id, passes a CSS colour through and returns null when absent", () => {
    expect(resolveColor("1")).toBe(PRESET_COLORS["1"]);
    expect(resolveColor("#abcdef")).toBe("#abcdef");
    expect(resolveColor(undefined)).toBeNull();
  });
});

describe("themeFromTokens", () => {
  it("falls back to the reference palette without a themed element", () => {
    expect(themeFromTokens(null, "dark")).toEqual(FALLBACK_THEMES.dark);
    expect(themeFromTokens(null, "light")).toEqual(FALLBACK_THEMES.light);
  });
});
