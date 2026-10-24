/**
 * The document theme, read once and shared.
 *
 * `<html data-theme>` is the app's own answer (`ThemeProvider` keeps it in sync), and it is
 * what a canvas or a WebGL surface has to follow: neither can read a CSS custom property
 * through the DOM. The attribute is observable, so one small subscription serves every
 * surface that has to re-render when the theme changes.
 */
import { useSyncExternalStore } from "react";

/** subscribeThemeAttr calls `onChange` whenever the document's theme attribute changes. */
export function subscribeThemeAttr(onChange: () => void): () => void {
  if (typeof MutationObserver === "undefined" || typeof document === "undefined") return () => {};
  const obs = new MutationObserver(onChange);
  obs.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
  return () => obs.disconnect();
}

/** readThemeAttr is the theme the document declares, or null when it declares none. */
export function readThemeAttr(): "light" | "dark" | null {
  if (typeof document === "undefined") return null;
  const attr = document.documentElement.dataset.theme;
  return attr === "light" || attr === "dark" ? attr : null;
}

/**
 * useDocumentTheme is the theme in force, following the attribute; before the attribute is
 * set it falls back to the system preference so a frame painted early is not the wrong one.
 */
export function useDocumentTheme(): "light" | "dark" {
  const attr = useSyncExternalStore(subscribeThemeAttr, readThemeAttr, () => null);
  if (attr) return attr;
  if (typeof matchMedia !== "undefined" && matchMedia("(prefers-color-scheme: dark)").matches) {
    return "dark";
  }
  return "light";
}
