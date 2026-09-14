import { NavLink } from "react-router-dom";
import { THEME_LABEL } from "../lib/theme";
import { useTheme } from "../lib/useTheme";

export function TopBar() {
  const { pref, cycle } = useTheme();
  return (
    <header className="top-bar">
      <span className="top-bar__brand">sdt viewer</span>
      <nav className="top-bar__tabs" aria-label="Primary">
        <NavLink
          to="/docs"
          end
          className={({ isActive }) => `top-bar__tab${isActive ? " is-active" : ""}`}
        >
          Documents
        </NavLink>
        <NavLink
          to="/wiki"
          className={({ isActive }) => `top-bar__tab${isActive ? " is-active" : ""}`}
        >
          Wiki
        </NavLink>
        <NavLink
          to="/search"
          className={({ isActive }) => `top-bar__tab${isActive ? " is-active" : ""}`}
        >
          Search
        </NavLink>
      </nav>
      <div className="top-bar__actions">
        <button
          type="button"
          className="theme-toggle"
          onClick={cycle}
          aria-label={`Theme: ${pref}, click to change`}
          title={`Theme: ${pref} — click to cycle`}
        >
          <span aria-hidden="true">{pref === "light" ? "☼" : pref === "dark" ? "☾" : "◐"}</span>
          <span>{THEME_LABEL(pref)}</span>
        </button>
      </div>
    </header>
  );
}
