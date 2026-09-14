import { createContext } from "react";
import type { AppliedTheme, ThemePreference } from "./theme";

export interface ThemeState {
  pref: ThemePreference;
  applied: AppliedTheme;
  systemDark: boolean;
  cycle: () => void;
}

export const ThemeContext = createContext<ThemeState | null>(null);
