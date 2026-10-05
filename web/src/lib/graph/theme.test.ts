// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { graphBackdrop, GRAPH_BACKDROP_CSS } from "./theme";
import { DEFAULT_BG_STOPS } from "./colors";

afterEach(() => {
  document.documentElement.style.removeProperty("--bg-base");
  document.documentElement.style.removeProperty("--bg-mantle");
  document.documentElement.style.removeProperty("--bg-crust");
});

describe("graphBackdrop", () => {
  it("falls back to the default stops when the tokens are absent", () => {
    const b = graphBackdrop();
    expect(b.stops).toEqual(DEFAULT_BG_STOPS);
    expect(b.css).toContain(DEFAULT_BG_STOPS[0]);
  });

  it("resolves the stops from the theme tokens", () => {
    document.documentElement.style.setProperty("--bg-base", "#1e1e2e");
    document.documentElement.style.setProperty("--bg-mantle", "#181825");
    document.documentElement.style.setProperty("--bg-crust", "#11111b");
    const b = graphBackdrop();
    expect(b.stops).toEqual(["#1e1e2e", "#181825", "#11111b"]);
    expect(b.css).toBe(
      "radial-gradient(1200px 800px at 50% 38%, #1e1e2e 0%, #181825 58%, #11111b 100%)",
    );
  });

  it("exposes a live var-based backdrop for the canvas", () => {
    expect(GRAPH_BACKDROP_CSS).toContain("var(--bg-base");
    expect(GRAPH_BACKDROP_CSS).toContain("var(--bg-crust");
  });
});
