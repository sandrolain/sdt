/**
 * WikiBoardView — the `/wiki/board` surface.
 *
 * Mounts a board-scoped `DockviewReact` (centre = the shared `JsonCanvas`, right
 * edge = `BoardControls`) and owns the data fetch, the source selection and the
 * view state (mode, layer visibility, minimap, zoom). The canvas carries no
 * chrome; every control is a sidebar control (analysis Behaviour decisions 3/4,
 * decision 0025).
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
import { BoardControls, type BoardLayer } from "./BoardControls";
import { SkeletonLines } from "./Skeleton";
import { useReloadToken } from "../lib/useReloadToken";
import { useOpenDocs } from "../lib/openDocsContext";

export const WIKI_BOARD_STORAGE_KEY = "wiki-board";

/** Dockview needs real layout measurement; tests use a plain columns fallback. */
const DOCKVIEW_ENABLED = import.meta.env.MODE !== "test";

interface BoardWorkspaceValue {
  model: BoardModel;
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
  const { model, mode, hiddenLayers, showMinimap, zoom, setZoom, setViewHandle, open } =
    useBoardWorkspace();
  return (
    <div className="board-panel">
      <JsonCanvas
        ref={setViewHandle}
        data={model}
        mode={mode}
        hiddenLayers={hiddenLayers}
        showMinimap={showMinimap}
        fitKey={model}
        onViewChange={setZoom}
        onOpenNode={open}
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

  const layers = useMemo<BoardLayer[]>(() => (model ? canvasLayers(model) : []), [model]);

  const toggleLayer = useCallback((id: number) => {
    setHiddenLayers((h) => (h.includes(id) ? h.filter((x) => x !== id) : [...h, id]));
  }, []);

  const open = useCallback(
    (node: BoardNode) => {
      if (node.file) {
        const path = node.file.startsWith("context/") ? node.file : `context/${node.file}`;
        openDoc(path);
        return;
      }
      if (node.type === "text" && node.id) openDoc(`context/wiki/${node.id}.md`);
    },
    [openDoc],
  );

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
  if (!model) return <SkeletonLines count={5} label="Loading board" />;

  const value: BoardWorkspaceValue = {
    model,
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
