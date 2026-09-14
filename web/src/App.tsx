import { Navigate, Route, Routes } from "react-router-dom";
import { TopBar } from "./components/TopBar";
import { ThemeProvider } from "./components/ThemeProvider";
import { DocumentsPage } from "./pages/DocumentsPage";
import {
  WikiPage,
  WikiGraphPlaceholder,
  WikiBoardPlaceholder,
  WikiPagePlaceholder,
} from "./pages/WikiPage";
import { SearchPage } from "./pages/SearchPage";

export function App() {
  return (
    <ThemeProvider>
      <div className="app-shell">
        <TopBar />
        <Routes>
          <Route path="/" element={<Navigate to="/docs" replace />} />
          <Route path="/docs" element={<DocumentsPage />} />
          <Route path="/docs/*" element={<DocumentsPage />} />
          <Route path="/wiki" element={<WikiPage />}>
            <Route index element={<Navigate to="/wiki/graph" replace />} />
            <Route path="graph" element={<WikiGraphPlaceholder />} />
            <Route path="board" element={<WikiBoardPlaceholder />} />
            <Route path=":id" element={<WikiPagePlaceholder />} />
          </Route>
          <Route path="/search" element={<SearchPage />} />
          <Route path="*" element={<p className="content__empty">not found</p>} />
        </Routes>
      </div>
    </ThemeProvider>
  );
}
