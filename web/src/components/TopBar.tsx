import { NavLink, useLocation } from "react-router-dom";
import { Icon } from "../lib/icon";
import { THEME_LABEL } from "../lib/theme";
import { useTheme } from "../lib/useTheme";
import { ReadingSettings } from "./ReadingSettings";

interface TopBarProps {
  onOpenSearch: () => void;
}

const IS_MAC =
  typeof navigator !== "undefined" &&
  /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);

/** One icon idiom (V3): the theme state is a Material Symbols ligature like
 *  every other icon in the shell, not a literal glyph. */
const THEME_ICON: Record<string, string> = {
  light: "light_mode",
  dark: "dark_mode",
  system: "contrast",
};

export function TopBar({ onOpenSearch }: TopBarProps) {
  const { pref, cycle } = useTheme();
  const { pathname } = useLocation();
  // The Board entry preserves the current board route (source + drill path).
  const boardTo = pathname.startsWith("/wiki/board") ? pathname : "/wiki/board";
  return (
    <header className="top-bar">
      <div className="top-bar__brand">
        <img src="/sdt-logo.svg" alt="SDT" className="top-bar__logo" />
        <span>sdt viewer</span>
      </div>
      <nav className="top-bar__tabs" aria-label="Primary">
        <div className="top-bar__group">
          <NavLink
            to="/docs"
            className={({ isActive }) =>
              `top-bar__tab${isActive && !pathname.startsWith("/docs/map") ? " is-active" : ""}`
            }
          >
            <Icon name="description" />
            Documents
          </NavLink>
          <NavLink
            to="/docs/map"
            end
            className={({ isActive }) => `top-bar__tab${isActive ? " is-active" : ""}`}
          >
            <Icon name="scatter_plot" />
            Map
          </NavLink>
        </div>
        <div className="top-bar__group">
          <span className="top-bar__group-label">Wiki</span>
          <NavLink
            to="/wiki/graph"
            className={({ isActive }) => `top-bar__tab${isActive ? " is-active" : ""}`}
          >
            <Icon name="hub" />
            Graph
          </NavLink>
          <NavLink
            to={boardTo}
            className={({ isActive }) => `top-bar__tab${isActive ? " is-active" : ""}`}
          >
            <Icon name="dashboard" />
            Board
          </NavLink>
        </div>
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
        <ReadingSettings />
        <button
          type="button"
          className="theme-toggle"
          onClick={cycle}
          aria-label={`Theme: ${pref}, click to change`}
          title={`Theme: ${pref} — click to cycle`}
        >
          <span className="ms-icon" aria-hidden="true">
            {THEME_ICON[pref] ?? "contrast"}
          </span>
          <span>{THEME_LABEL(pref)}</span>
        </button>
      </div>
    </header>
  );
}
