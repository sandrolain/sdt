import { useContext } from "react";
import { ThemeContext, type ThemeState } from "./theme-context";

export function useTheme(): ThemeState {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used inside ThemeProvider");
  return ctx;
}

/** The theme state when a provider is above, null when there is none. A surface
 *  that must render outside the app shell (a harness, a focused test) uses this
 *  instead of the strict hook, and hides the affordance it cannot honour. */
export function useThemeOptional(): ThemeState | null {
  return useContext(ThemeContext);
}
