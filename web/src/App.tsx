import { useEffect, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { TopBar } from "./components/TopBar";
import { ThemeProvider } from "./components/ThemeProvider";
import { SearchPalette } from "./components/SearchPalette";
import { DocumentsPage } from "./pages/DocumentsPage";
import { WikiPage } from "./pages/WikiPage";
import { WikiGraphView } from "./components/WikiGraphView";
import { WikiBoardView } from "./components/WikiBoardView";
import { WikiPageDetail } from "./components/WikiPageDetail";

export function App() {
  const [searchOpen, setSearchOpen] = useState(false);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setSearchOpen((open) => !open);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  return (
    <ThemeProvider>
      <div className="app-shell">
        <TopBar onOpenSearch={() => setSearchOpen(true)} />
        <Routes>
          <Route path="/" element={<Navigate to="/docs" replace />} />
          <Route path="/docs" element={<DocumentsPage />} />
          <Route path="/docs/*" element={<DocumentsPage />} />
          <Route path="/wiki" element={<WikiPage />}>
            <Route index element={<Navigate to="/wiki/graph" replace />} />
            <Route path="graph" element={<WikiGraphView />} />
            <Route path="board" element={<WikiBoardView />} />
            <Route path="*" element={<WikiPageDetail />} />
          </Route>
          <Route path="*" element={<p className="content__empty">not found</p>} />
        </Routes>
        <SearchPalette open={searchOpen} onOpenChange={setSearchOpen} />
      </div>
    </ThemeProvider>
  );
}
