import { NavLink, Outlet } from "react-router-dom";

export function WikiPage() {
  return (
    <div className="wiki-shell">
      <nav className="wiki-modes" aria-label="Wiki mode">
        <NavLink
          to="/wiki/graph"
          className={({ isActive }) => `wiki-mode${isActive ? " is-active" : ""}`}
        >
          Graph
        </NavLink>
        <NavLink
          to="/wiki/board"
          className={({ isActive }) => `wiki-mode${isActive ? " is-active" : ""}`}
        >
          Board
        </NavLink>
      </nav>
      <main className="content">
        <Outlet />
      </main>
    </div>
  );
}
