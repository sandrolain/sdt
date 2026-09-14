export type ThemePreference = "light" | "dark" | "system";
export type AppliedTheme = "light" | "dark";
export type Theme = AppliedTheme;

export const THEME_KEY = "sdt-theme";

export const THEME_LABEL = (p: ThemePreference) =>
  p === "light" ? "light" : p === "dark" ? "dark" : "system";

/** Resolve the applied theme from a preference and the OS preference. */
export function pickTheme(pref: ThemePreference, systemDark: boolean): AppliedTheme {
  if (pref === "system") return systemDark ? "dark" : "light";
  return pref;
}

/** Cycle light → dark → system → light. */
export function nextTheme(pref: ThemePreference): ThemePreference {
  return pref === "light" ? "dark" : pref === "dark" ? "system" : "light";
}

/** Read a stored preference, defaulting to system. */
export function readPreference(): ThemePreference {
  const raw = typeof localStorage !== "undefined" ? localStorage.getItem(THEME_KEY) : null;
  return raw === "light" || raw === "dark" || raw === "system" ? raw : "system";
}

/** Persist a preference and apply it to <html data-theme>. */
export function applyTheme(pref: ThemePreference, systemDark: boolean) {
  const applied = pickTheme(pref, systemDark);
  const root = document.documentElement;
  localStorage.setItem(THEME_KEY, pref);
  root.dataset.theme = applied;
  root.dataset.themePref = pref;
}
