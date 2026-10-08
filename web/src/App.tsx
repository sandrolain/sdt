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
import { SemanticMapView } from "./components/SemanticMapView";
import { OpenDocsProvider } from "./components/OpenDocsProvider";
import { applyLiveChange, connectLiveUpdates } from "./lib/liveUpdates";
import { applyReadingPrefs, currentReadingPrefs } from "./lib/readingPrefs";
import { requestTreeFocus } from "./lib/treeFocus";
import { isTreeFocusKey } from "./lib/treeKeyboard";

export function App() {
  const [searchOpen, setSearchOpen] = useState(false);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setSearchOpen((open) => !open);
        return;
      }
      // ⌘\ / Ctrl+\ pulls focus into the documents tree
      if (isTreeFocusKey(e)) {
        e.preventDefault();
        requestTreeFocus();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  useEffect(() => connectLiveUpdates(() => applyLiveChange()), []);

  // publish the stored reading preferences on the root before the first
  // document paints, so a reload keeps its text size and measure
  useEffect(() => {
    applyReadingPrefs(currentReadingPrefs());
  }, []);

  return (
    <ThemeProvider>
      <OpenDocsProvider>
        <div className="app-shell">
          <TopBar onOpenSearch={() => setSearchOpen(true)} />
          <Routes>
            <Route path="/" element={<Navigate to="/docs" replace />} />
            <Route path="/docs" element={<DocumentsPage />} />
            <Route path="/docs/map" element={<SemanticMapView />} />
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
      </OpenDocsProvider>
    </ThemeProvider>
  );
}
