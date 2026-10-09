import { Outlet } from "react-router-dom";

export function WikiPage() {
  return (
    <div className="wiki-shell">
      <main className="content">
        <Outlet />
      </main>
    </div>
  );
}
