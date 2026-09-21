import { NavLink } from "react-router-dom";
import { Icon } from "../lib/icon";
import { THEME_LABEL } from "../lib/theme";
import { useTheme } from "../lib/useTheme";

interface TopBarProps {
  onOpenSearch: () => void;
}

const IS_MAC =
  typeof navigator !== "undefined" &&
  /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);

export function TopBar({ onOpenSearch }: TopBarProps) {
  const { pref, cycle } = useTheme();
  return (
    <header className="top-bar">
      <div className="top-bar__brand">
        <img src="/sdt-logo.svg" alt="SDT" className="top-bar__logo" />
        <span>sdt viewer</span>
      </div>
      <nav className="top-bar__tabs" aria-label="Primary">
        <NavLink
          to="/docs"
          className={({ isActive }) => `top-bar__tab${isActive ? " is-active" : ""}`}
        >
          <Icon name="description" />
          Documents
        </NavLink>
        <NavLink
          to="/wiki"
          className={({ isActive }) => `top-bar__tab${isActive ? " is-active" : ""}`}
        >
          <Icon name="menu_book" />
          Wiki
        </NavLink>
      </nav>
      <div className="top-bar__actions">
        <button
          type="button"
          className="search-trigger"
          onClick={onOpenSearch}
          aria-label="Search the corpus"
        >
          <span>Search</span>
          <kbd className="search-trigger__kbd">{IS_MAC ? "⌘K" : "Ctrl K"}</kbd>
        </button>
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
