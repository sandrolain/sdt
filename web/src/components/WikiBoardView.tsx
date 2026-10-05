/**
 * WikiBoardView — the `/wiki/board` surface.
 *
 * Mounts a board-scoped `DockviewReact` (centre = the shared `JsonCanvas`, right
 * edge = `BoardControls`) and owns the data fetch, the source selection, the
 * view state (mode, layer visibility, minimap, zoom) and the nested-canvas
 * drill-down (decision 0025; analysis 20261003-224537). The canvas carries no
 * chrome; every control is a sidebar control.
 */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useSearchParams } from "react-router-dom";
import {
  DockviewReact,
  themeCatppuccinMochaSpaced,
  type DockviewApi,
  type DockviewReadyEvent,
  type IDockviewPanelProps,
} from "dockview-react";
import { fetchTree, fetchWikiBoard, type TreeEntry } from "../lib/api";
import { canvasLayers } from "../lib/jsoncanvas/document";
import { clearLayout, loadLayout, saveLayout } from "../lib/layoutStore";
import { normalizeBoard, type BoardModel, type BoardNode } from "../lib/canvas";
import { displayTitle } from "../lib/titles";
import { addBoardPanels, BOARD_CENTER_PANEL_ID } from "../lib/wikiEdgeLayout";
import { JsonCanvas, type JsonCanvasHandle } from "./JsonCanvas";
import { PLACEHOLDER_SCALE, renderNestedBody } from "./nestedCanvasBody";
import { BoardControls, type BoardLayer, type BoardCrumb } from "./BoardControls";
import { SkeletonLines } from "./Skeleton";
import { useReloadToken } from "../lib/useReloadToken";
import { useOpenDocs } from "../lib/openDocsContext";

export const WIKI_BOARD_STORAGE_KEY = "wiki-board";

/** Dockview needs real layout measurement; tests use a plain columns fallback. */
const DOCKVIEW_ENABLED = import.meta.env.MODE !== "test";

/** One entered nested-canvas level (in-place drill-down, no DOM reparenting). */
interface BoardLevel {
  nodeId: string;
  label: string;
  model: BoardModel;
}

interface BoardWorkspaceValue {
  /** The active level's document (root, or the deepest entered nested canvas). */
  document: BoardModel;
  sources: { path: string; label: string }[];
  source: string;
  onSource: (path: string) => void;
  mode: "2d" | "3d";
  setMode: (mode: "2d" | "3d") => void;
  hiddenLayers: number[];
  toggleLayer: (id: number) => void;
  showMinimap: boolean;
  setShowMinimap: (value: boolean) => void;
  zoom: number;
  setZoom: (zoom: number) => void;
  zoomIn: () => void;
  zoomOut: () => void;
  fit: () => void;
  layers: BoardLayer[];
  crumbs: BoardCrumb[];
  onCrumb: (id: string | null) => void;
  setViewHandle: (handle: JsonCanvasHandle | null) => void;
  open: (node: BoardNode) => void;
}

const BoardWorkspaceContext = createContext<BoardWorkspaceValue | null>(null);

function useBoardWorkspace(): BoardWorkspaceValue {
  const value = useContext(BoardWorkspaceContext);
  if (!value) throw new Error("useBoardWorkspace must be used inside WikiBoardView");
  return value;
}

function BoardPanel() {
  const {
    document: doc,
    mode,
    hiddenLayers,
    showMinimap,
    zoom,
    setZoom,
    setViewHandle,
    open,
  } = useBoardWorkspace();
  return (
    <div className="board-panel">
      <JsonCanvas
        ref={setViewHandle}
        data={doc}
        mode={mode}
        hiddenLayers={hiddenLayers}
        showMinimap={showMinimap}
        fitKey={doc}
        placeholderScale={PLACEHOLDER_SCALE}
        onViewChange={setZoom}
        onOpenNode={open}
        renderNode={(node) => renderNestedBody(node, zoom)}
      />
      <span className="visually-hidden" aria-live="polite">
        Zoom {Math.round(zoom * 100)}%
      </span>
    </div>
  );
}

function BoardControlsPanel() {
  const b = useBoardWorkspace();
  return (
    <div className="dock-content board-controls-panel">
      <BoardControls
        sources={b.sources}
        source={b.source}
        onSource={b.onSource}
        crumbs={b.crumbs}
        onCrumb={b.onCrumb}
        zoom={b.zoom}
        onZoomIn={b.zoomIn}
        onZoomOut={b.zoomOut}
        onFit={b.fit}
        mode={b.mode}
        onMode={b.setMode}
        layers={b.layers}
        hiddenLayers={b.hiddenLayers}
        onToggleLayer={b.toggleLayer}
        showMinimap={b.showMinimap}
        onShowMinimap={b.setShowMinimap}
      />
    </div>
  );
}

/** Stable dockview component registry (module scope: never recreated). */
const components: Record<string, (props: IDockviewPanelProps) => ReactNode> = {
  "json-canvas": () => <BoardPanel />,
  "board-controls": () => <BoardControlsPanel />,
};

/** Read-only board from the graph default or a selected `.canvas` file. */
export function WikiBoardView() {
  const [params, setParams] = useSearchParams();
  const { open: openDoc } = useOpenDocs();
  const file = params.get("file") ?? "";
  const [model, setModel] = useState<BoardModel | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [canvases, setCanvases] = useState<TreeEntry[]>([]);
  const [mode, setMode] = useState<"2d" | "3d">("2d");
  const [levels, setLevels] = useState<BoardLevel[]>([]);
  const [hiddenLayers, setHiddenLayers] = useState<number[]>([]);
  const [showMinimap, setShowMinimap] = useState(true);
  const [zoom, setZoom] = useState(1);
  const viewRef = useRef<JsonCanvasHandle | null>(null);
  const apiRef = useRef<DockviewApi | null>(null);
  const reloadToken = useReloadToken();

  useEffect(() => {
    let alive = true;
    fetchTree()
      .then((res) => {
        if (alive) setCanvases(res.entries.filter((e) => e.canvas));
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [reloadToken]);

  useEffect(() => {
    let alive = true;
    fetchWikiBoard(file || undefined)
      .then((res) => {
        if (alive) {
          setModel(normalizeBoard(res));
          // A new source (or reload) resets the drill-down to the root level.
          setLevels([]);
          setError(null);
        }
      })
      .catch((err: unknown) => {
        if (alive) {
          setModel(null);
          setError(err instanceof Error ? err.message : String(err));
        }
      });
    return () => {
      alive = false;
    };
  }, [file, reloadToken]);

  const setViewHandle = useCallback((handle: JsonCanvasHandle | null) => {
    viewRef.current = handle;
  }, []);

  const onSource = useCallback(
    (path: string) => {
      const next = new URLSearchParams(params);
      if (path) next.set("file", path);
      else next.delete("file");
      setParams(next, { replace: true });
    },
    [params, setParams],
  );

  const sources = useMemo(
    () =>
      canvases.map((c) => ({
        path: c.path,
        label: displayTitle({ title: c.title, path: c.path }),
      })),
    [canvases],
  );

  const activeDocument = levels.length > 0 ? levels[levels.length - 1].model : model;
  const layers = useMemo<BoardLayer[]>(
    () => (activeDocument ? canvasLayers(activeDocument) : []),
    [activeDocument],
  );

  const toggleLayer = useCallback((id: number) => {
    setHiddenLayers((h) => (h.includes(id) ? h.filter((x) => x !== id) : [...h, id]));
  }, []);

  const sourceLabel = file ? displayTitle({ path: file }) : "Wiki graph";
  const crumbs = useMemo<BoardCrumb[]>(
    () => [
      { id: "", label: sourceLabel },
      ...levels.map((l) => ({ id: l.nodeId, label: l.label })),
    ],
    [sourceLabel, levels],
  );

  const onCrumb = useCallback((id: string | null) => {
    setLevels((prev) => {
      if (id === null || id === "") return [];
      const index = prev.findIndex((l) => l.nodeId === id);
      return index >= 0 ? prev.slice(0, index + 1) : prev;
    });
  }, []);

  const open = useCallback(
    (node: BoardNode) => {
      if (node.type === "nested-canvas" && node.canvas) {
        setLevels((prev) => [
          ...prev,
          { nodeId: node.id, label: String(node.title ?? "Nested canvas"), model: node.canvas! },
        ]);
        return;
      }
      // An external `.canvas` target reuses the `?file=` board flow.
      const canvasTarget =
        (node.file && node.file.endsWith(".canvas") && node.file) ||
        (node.url && node.url.endsWith(".canvas") && node.url) ||
        "";
      if (canvasTarget) {
        const path = canvasTarget.startsWith("context/") ? canvasTarget : `context/${canvasTarget}`;
        onSource(path);
        return;
      }
      if (node.file) {
        const path = node.file.startsWith("context/") ? node.file : `context/${node.file}`;
        openDoc(path);
        return;
      }
      if (node.type === "text" && node.id) openDoc(`context/wiki/${node.id}.md`);
    },
    [openDoc, onSource],
  );

  // Escape pops one drill-down level (the view keeps Escape for its selection).
  useEffect(() => {
    if (levels.length === 0) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setLevels((prev) => prev.slice(0, -1));
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [levels.length]);

  const zoomIn = useCallback(() => viewRef.current?.zoomIn(), []);
  const zoomOut = useCallback(() => viewRef.current?.zoomOut(), []);
  const fit = useCallback(() => viewRef.current?.fit(), []);

  const onReady = useCallback((event: DockviewReadyEvent) => {
    const api = event.api;
    apiRef.current = api;
    const stored = loadLayout(WIKI_BOARD_STORAGE_KEY);
    if (stored) {
      try {
        api.fromJSON(stored as never);
      } catch {
        clearLayout(WIKI_BOARD_STORAGE_KEY);
      }
    }
    if (!api.getPanel(BOARD_CENTER_PANEL_ID)) {
      api.addPanel({
        id: BOARD_CENTER_PANEL_ID,
        component: "json-canvas",
        title: "Board",
        minimumWidth: 320,
      });
    }
    addBoardPanels(api);
    api.onDidLayoutChange(() => saveLayout(WIKI_BOARD_STORAGE_KEY, api.toJSON()));
  }, []);

  if (error) return <p className="content__empty">Board error: {error}</p>;
  if (!model || !activeDocument) return <SkeletonLines count={5} label="Loading board" />;

  const value: BoardWorkspaceValue = {
    document: activeDocument,
    sources,
    source: file,
    onSource,
    mode,
    setMode,
    hiddenLayers,
    toggleLayer,
    showMinimap,
    setShowMinimap,
    zoom,
    setZoom,
    zoomIn,
    zoomOut,
    fit,
    layers,
    crumbs,
    onCrumb,
    setViewHandle,
    open,
  };

  if (!DOCKVIEW_ENABLED) {
    return (
      <BoardWorkspaceContext.Provider value={value}>
        <div
          className="dock-layout dock-layout--fallback wiki-board-fallback"
          data-testid="wiki-board"
        >
          <section className="dock-content" aria-label="Board">
            <BoardPanel />
          </section>
          <section className="dock-content" aria-label="Board controls">
            <BoardControlsPanel />
          </section>
        </div>
      </BoardWorkspaceContext.Provider>
    );
  }

  return (
    <BoardWorkspaceContext.Provider value={value}>
      <DockviewReact
        className="wiki-workspace dock-layout"
        theme={themeCatppuccinMochaSpaced}
        components={components}
        onReady={onReady}
      />
    </BoardWorkspaceContext.Provider>
  );
}
