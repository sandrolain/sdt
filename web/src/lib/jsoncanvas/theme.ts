/**
 * Theme for the shared JSON Canvas view.
 *
 * The 2D canvas and the 3D texture painter both need concrete colours, so the
 * theme is read from the SDT semantic CSS custom properties at runtime and
 * re-derived on a `data-theme` change (the view listens). When a token is absent
 * (jsdom, an unthemed host) the reference's light/dark palette is the fallback.
 * Preset `1`-`6` stay raw categorical identities.
 */
import type { CanvasNode } from "./document";

export const PRESET_COLORS: Record<string, string> = {
  1: "#e5484d",
  2: "#f08a3c",
  3: "#e5b82e",
  4: "#2fa56b",
  5: "#16a9c7",
  6: "#8d5bd6",
};

export interface CanvasTheme {
  bg: string;
  grid: string;
  node: string;
  text: string;
  muted: string;
  border: string;
  accent: string;
  edge: string;
  group: string;
}

export const FALLBACK_THEMES: { light: CanvasTheme; dark: CanvasTheme } = {
  light: {
    bg: "#f2f4f8",
    grid: "#d3d9e5",
    node: "#ffffff",
    text: "#1b2333",
    muted: "#66718a",
    border: "#c9d1e0",
    accent: "#4863e8",
    edge: "#8590a8",
    group: "rgba(72,99,232,.05)",
  },
  dark: {
    bg: "#0d121c",
    grid: "#222a3a",
    node: "#161d2b",
    text: "#e6eaf3",
    muted: "#8d98b2",
    border: "#2c364a",
    accent: "#7f95ff",
    edge: "#63708c",
    group: "rgba(127,149,255,.06)",
  },
};

/** SDT semantic token per theme slot. */
const TOKEN_VARS: Record<keyof CanvasTheme, string> = {
  bg: "--bg-base",
  grid: "--border",
  node: "--bg-mantle",
  text: "--fg",
  muted: "--fg-dim",
  border: "--border",
  accent: "--accent",
  edge: "--fg-dim",
  group: "--accent",
};

/** Resolve a preset `1`-`6` or pass any CSS colour through; null when absent. */
export function resolveColor(
  c: string | undefined,
  presets: Record<string, string> = PRESET_COLORS,
): string | null {
  return c ? presets[c] || c : null;
}

function hexToRgba(hex: string, alpha: number): string | null {
  const m = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return null;
  const h =
    m[1].length === 3
      ? m[1]
          .split("")
          .map((c) => c + c)
          .join("")
      : m[1];
  const r = parseInt(h.slice(0, 2), 16);
  const g = parseInt(h.slice(2, 4), 16);
  const b = parseInt(h.slice(4, 6), 16);
  return `rgba(${r},${g},${b},${alpha})`;
}

/** Concrete theme for the current mode, read from the semantic tokens. */
export function themeFromTokens(el: HTMLElement | null, mode: "light" | "dark"): CanvasTheme {
  const base = FALLBACK_THEMES[mode];
  if (!el || typeof getComputedStyle === "undefined") return base;
  const styles = getComputedStyle(el);
  const read = (slot: keyof CanvasTheme): string | null => {
    const value = styles.getPropertyValue(TOKEN_VARS[slot]).trim();
    return value || null;
  };
  const accent = read("accent") ?? base.accent;
  const group = hexToRgba(accent, mode === "light" ? 0.06 : 0.08) ?? base.group;
  return {
    bg: read("bg") ?? base.bg,
    grid: read("grid") ?? base.grid,
    node: read("node") ?? base.node,
    text: read("text") ?? base.text,
    muted: read("muted") ?? base.muted,
    border: read("border") ?? base.border,
    accent,
    edge: read("edge") ?? base.edge,
    group,
  };
}

/** Sanitized 3D label text for a node (strips markdown markers). */
export function nodeLabelText(n: CanvasNode): string {
  const raw =
    n.type === "text"
      ? n.text
      : n.type === "file"
        ? n.file
        : n.type === "link"
          ? n.url
          : n.type === "nested-canvas"
            ? n.title
            : n.label;
  return String(raw ?? "").replace(/[#*`]/g, "");
}
