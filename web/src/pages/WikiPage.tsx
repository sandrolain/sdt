import { NavLink, Outlet, useLocation } from "react-router-dom";
import { Icon } from "../lib/icon";

export function WikiPage() {
  const { pathname } = useLocation();
  // The Board tab preserves the current board route (source + drill path).
  const boardTo = pathname.startsWith("/wiki/board") ? pathname : "/wiki/board";
  return (
    <div className="wiki-shell">
      <nav className="wiki-modes" aria-label="Wiki mode">
        <NavLink
          to="/wiki/graph"
          className={({ isActive }) => `wiki-mode${isActive ? " is-active" : ""}`}
        >
          <Icon name="hub" />
          Graph
        </NavLink>
        <NavLink
          to={boardTo}
          className={({ isActive }) => `wiki-mode${isActive ? " is-active" : ""}`}
        >
          <Icon name="dashboard" />
          Board
        </NavLink>
      </nav>
      <main className="content">
        <Outlet />
      </main>
    </div>
  );
}
