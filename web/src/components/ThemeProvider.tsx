import { useCallback, useEffect, useState, type ReactNode } from "react";
import {
  applyTheme,
  nextTheme,
  pickTheme,
  readPreference,
  type ThemePreference,
} from "../lib/theme";
import { ThemeContext } from "../lib/theme-context";

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [pref, setPref] = useState<ThemePreference>(readPreference);
  const [systemDark, setSystemDark] = useState(
    () => window.matchMedia("(prefers-color-scheme: dark)").matches,
  );

  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = (e: MediaQueryListEvent) => setSystemDark(e.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const cycle = useCallback(() => setPref((p) => nextTheme(p)), []);

  const applied = pickTheme(pref, systemDark);

  // Keep <html data-theme> in sync whenever preference or OS theme changes.
  useEffect(() => {
    applyTheme(pref, systemDark);
  }, [pref, systemDark]);

  return (
    <ThemeContext.Provider value={{ pref, applied, systemDark, cycle }}>
      {children}
    </ThemeContext.Provider>
  );
}
