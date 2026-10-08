import { Suspense, lazy, useEffect, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { TopBar } from "./components/TopBar";
import { ThemeProvider } from "./components/ThemeProvider";
import { SearchPalette } from "./components/SearchPalette";
import { DocumentsPage } from "./pages/DocumentsPage";
import { WikiPage } from "./pages/WikiPage";
import { WikiBoardView } from "./components/WikiBoardView";
import { WikiPageDetail } from "./components/WikiPageDetail";
import { OpenDocsProvider } from "./components/OpenDocsProvider";
import { applyLiveChange, connectLiveUpdates } from "./lib/liveUpdates";
import { applyReadingPrefs, currentReadingPrefs } from "./lib/readingPrefs";
import { requestTreeFocus } from "./lib/treeFocus";
import { isTreeFocusKey } from "./lib/treeKeyboard";

// The ported three.js graph engine (ADR-0024) is heavy and only needed on the
// graph and map routes; lazy-load them so `three` stays out of the entry chunk.
const WikiGraphView = lazy(() =>
  import("./components/WikiGraphView").then((m) => ({ default: m.WikiGraphView })),
);
const SemanticMapView = lazy(() =>
  import("./components/SemanticMapView").then((m) => ({ default: m.SemanticMapView })),
);

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
            <Route
              path="/docs/map"
              element={
                <Suspense fallback={<p className="content__empty">Loading map…</p>}>
                  <SemanticMapView />
                </Suspense>
              }
            />
            <Route path="/docs/*" element={<DocumentsPage />} />
            <Route path="/wiki" element={<WikiPage />}>
              <Route index element={<Navigate to="/wiki/graph" replace />} />
              <Route
                path="graph"
                element={
                  <Suspense fallback={<p className="content__empty">Loading graph…</p>}>
                    <WikiGraphView />
                  </Suspense>
                }
              />
              <Route path="board/*" element={<WikiBoardView />} />
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
