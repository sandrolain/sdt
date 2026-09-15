import { NavLink, Outlet } from "react-router-dom";

export function WikiPage() {
  return (
    <div className="workspace">
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
      <aside className="panel panel--related" aria-hidden="true">
        <div className="panel-header">
          <span className="panel-header__title">Inspector</span>
        </div>
      </aside>
    </div>
  );
}

export function WikiBoardPlaceholder() {
  return <p className="placeholder">Wiki board — Phase 10 (JSON Canvas).</p>;
}
