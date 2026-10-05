/**
 * Graph backdrop resolved from the Catppuccin theme tokens (analysis D1).
 *
 * The port deliberately deferred a per-theme backdrop (plan amendment (b));
 * this reader re-derives the reference's radial gradient from `--bg-base` /
 * `--bg-mantle` / `--bg-crust` at runtime, so the canvas and the SVG export
 * follow the active theme in both Latte and Mocha. `DEFAULT_BG_STOPS` are the
 * fallback only, not a second palette.
 */
import { DEFAULT_BG_STOPS } from "./colors";

function cssVar(name: string): string {
  if (typeof document === "undefined") return "";
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

/**
 * Canvas backdrop as a live CSS value: the browser resolves the theme
 * variables on every theme change, so the canvas follows `data-theme` without a
 * React re-render (the provider writes `data-theme` in an effect, after child
 * effects, so a render-time read would lag by one commit).
 */
export const GRAPH_BACKDROP_CSS =
  "radial-gradient(1200px 800px at 50% 38%, var(--bg-base, #101a2e) 0%, " +
  "var(--bg-mantle, #080d18) 58%, var(--bg-crust, #04070d) 100%)";

export interface GraphBackdrop {
  /** full CSS `background` value for the canvas host */
  css: string;
  /** the three resolved stops, shared with the SVG export */
  stops: [string, string, string];
}

export function graphBackdrop(): GraphBackdrop {
  const stops: [string, string, string] = [
    cssVar("--bg-base") || DEFAULT_BG_STOPS[0],
    cssVar("--bg-mantle") || DEFAULT_BG_STOPS[1],
    cssVar("--bg-crust") || DEFAULT_BG_STOPS[2],
  ];
  return {
    stops,
    css:
      `radial-gradient(1200px 800px at 50% 38%, ${stops[0]} 0%, ` +
      `${stops[1]} 58%, ${stops[2]} 100%)`,
  };
}
