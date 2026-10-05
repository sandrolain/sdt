/**
 * WikiGraphView — the `/wiki/graph` surface.
 *
 * Mounts a graph-scoped `DockviewReact` (centre = `GraphView`, right edge =
 * `GraphControls`) and owns the data fetch, the tools/selection/path state and
 * the SDT→engine adaptation. Replaces the former react-force-graph view
 * (analysis B1/B2/D1/D2/D4).
 */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
  type Dispatch,
  type ReactNode,
} from "react";
import {
  DockviewReact,
  themeCatppuccinMochaSpaced,
  type DockviewApi,
  type DockviewReadyEvent,
  type IDockviewPanelProps,
} from "dockview-react";
import { adaptToEngine, type AdaptedEngineGraph } from "../lib/graph/adapter";
import { findShortestPath, type ShortestPath } from "../lib/graph/path";
import { fetchWikiGraph, type GraphData } from "../lib/graphModel";
import {
  graphToolsReducer,
  initialGraphTools,
  visibleSet,
  type GraphToolsAction,
  type GraphToolsState,
} from "../lib/graphTools";
import { clearLayout, loadLayout, saveLayout } from "../lib/layoutStore";
import { addGraphPanels, GRAPH_CENTER_PANEL_ID } from "../lib/wikiEdgeLayout";
import { useOpenDocs } from "../lib/openDocsContext";
import { useReloadToken } from "../lib/useReloadToken";
import { GraphView, type GraphViewHandle } from "./GraphView";
import { GraphControls } from "./GraphControls";
import { SkeletonLines } from "./Skeleton";

export const WIKI_GRAPH_STORAGE_KEY = "wiki-graph";

/** Dockview needs real layout measurement; tests use a plain columns fallback. */
const DOCKVIEW_ENABLED = import.meta.env.MODE !== "test";

interface GraphWorkspaceValue {
  adapted: AdaptedEngineGraph;
  tools: GraphToolsState;
  dispatchTools: Dispatch<GraphToolsAction>;
  selectedId: string | null;
  select: (id: string | null) => void;
  path: ShortestPath | null;
  pathFrom: string;
  pathTo: string;
  setPathFrom: (id: string) => void;
  setPathTo: (id: string) => void;
  findPath: () => void;
  clearPath: () => void;
  clusters: { id: string; color: string; count: number }[];
  nodeOptions: { id: string; label: string }[];
  selectedTitle: string | null;
  setViewHandle: (handle: GraphViewHandle | null) => void;
  open: (id: string) => void;
  fit: () => void;
  clear: () => void;
  exportSVG: () => void;
  exportPNG: () => void;
}

const GraphWorkspaceContext = createContext<GraphWorkspaceValue | null>(null);

function useGraphWorkspace(): GraphWorkspaceValue {
  const value = useContext(GraphWorkspaceContext);
  if (!value) throw new Error("useGraphWorkspace must be used inside WikiGraphView");
  return value;
}

function GraphPanel() {
  const { setViewHandle, adapted, tools, selectedId, select, path } = useGraphWorkspace();
  return (
    <div className="graph-panel">
      <GraphView
        ref={setViewHandle}
        nodes={adapted.nodes}
        links={adapted.links}
        mode={tools.mode}
        layout={tools.layout}
        selectedId={selectedId}
        onSelect={select}
        highlightPath={path}
        labels={tools.showLabels ? "auto" : "none"}
        centrality={tools.centrality}
        neighborsOnly={tools.neighborsOnly}
      />
    </div>
  );
}

function GraphControlsPanel() {
  const g = useGraphWorkspace();
  return (
    <div className="dock-content graph-controls-panel">
      <GraphControls
        tools={g.tools}
        allVerbs={g.adapted.allVerbs}
        allKinds={g.adapted.allKinds}
        clusters={g.clusters}
        selectedId={g.selectedId}
        selectedTitle={g.selectedTitle}
        nodeOptions={g.nodeOptions}
        pathFrom={g.pathFrom}
        pathTo={g.pathTo}
        pathActive={g.path !== null}
        onTools={g.dispatchTools}
        onPathFrom={g.setPathFrom}
        onPathTo={g.setPathTo}
        onFindPath={g.findPath}
        onClearPath={g.clearPath}
        onFit={g.fit}
        onClear={g.clear}
        onOpen={g.open}
        onExportSVG={g.exportSVG}
        onExportPNG={g.exportPNG}
      />
    </div>
  );
}

/** Stable dockview component registry (module scope: never recreated). */
const components: Record<string, (props: IDockviewPanelProps) => ReactNode> = {
  graph: () => <GraphPanel />,
  "graph-controls": () => <GraphControlsPanel />,
};

export function WikiGraphView() {
  const [data, setData] = useState<GraphData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tools, dispatchTools] = useReducer(graphToolsReducer, initialGraphTools);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [path, setPath] = useState<ShortestPath | null>(null);
  const [pathFrom, setPathFrom] = useState("");
  const [pathTo, setPathTo] = useState("");
  const viewRef = useRef<GraphViewHandle | null>(null);
  const setViewHandle = useCallback((handle: GraphViewHandle | null) => {
    viewRef.current = handle;
  }, []);
  const apiRef = useRef<DockviewApi | null>(null);
  const { open: openDoc } = useOpenDocs();
  const reloadToken = useReloadToken();

  useEffect(() => {
    let alive = true;
    fetchWikiGraph()
      .then((d) => {
        if (alive) setData(d);
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      alive = false;
    };
  }, [reloadToken]);

  const allVerbs = useMemo(
    () => (data ? [...new Set(data.edges.map((e) => e.verb))].sort() : []),
    [data],
  );
  const allKinds = useMemo(
    () => (data ? [...new Set(data.edges.map((e) => e.kind))].sort() : []),
    [data],
  );

  const adapted = useMemo(
    () =>
      data
        ? adaptToEngine(data, {
            clusterKey: tools.clusterKey,
            visibleVerbs: visibleSet(allVerbs, tools.hiddenVerbs),
            visibleKinds: visibleSet(allKinds, tools.hiddenKinds),
          })
        : { nodes: [], links: [], palette: new Map(), allVerbs: [], allKinds: [] },
    [data, tools.clusterKey, tools.hiddenVerbs, tools.hiddenKinds, allVerbs, allKinds],
  );

  const clusters = useMemo(() => {
    const counts = new Map<string, number>();
    for (const n of adapted.nodes)
      counts.set(String(n.group), (counts.get(String(n.group)) ?? 0) + 1);
    return [...counts.entries()]
      .map(([id, count]) => ({ id, count, color: adapted.palette.get(id) ?? "#9399b2" }))
      .sort((a, b) => a.id.localeCompare(b.id));
  }, [adapted]);

  const nodeOptions = useMemo(
    () =>
      adapted.nodes
        .map((n) => ({ id: n.id, label: String(n.label ?? n.id) }))
        .sort((a, b) => a.label.localeCompare(b.label)),
    [adapted],
  );

  const selectedTitle = useMemo(() => {
    if (!selectedId) return null;
    return String(adapted.nodes.find((n) => n.id === selectedId)?.label ?? selectedId);
  }, [selectedId, adapted]);

  const open = useCallback(
    (id: string) => {
      openDoc(`context/wiki/${id}.md`);
    },
    [openDoc],
  );

  const fit = useCallback(() => viewRef.current?.fitView(), []);
  const clear = useCallback(() => {
    setSelectedId(null);
    setPath(null);
    viewRef.current?.fitView();
  }, []);
  const exportSVG = useCallback(() => viewRef.current?.exportSVG(), []);
  const exportPNG = useCallback(() => viewRef.current?.exportPNG(), []);

  const findPath = useCallback(() => {
    if (!pathFrom || !pathTo) return;
    const shortest = findShortestPath(pathFrom, pathTo, adapted.nodes, adapted.links);
    setPath(shortest);
    if (shortest) viewRef.current?.focusNode(pathFrom);
  }, [pathFrom, pathTo, adapted]);

  useEffect(() => {
    if (tools.layout) fit();
  }, [tools.layout, tools.clusterKey, fit]);

  const onReady = useCallback((event: DockviewReadyEvent) => {
    const api = event.api;
    apiRef.current = api;
    const stored = loadLayout(WIKI_GRAPH_STORAGE_KEY);
    if (stored) {
      try {
        api.fromJSON(stored as never);
      } catch {
        clearLayout(WIKI_GRAPH_STORAGE_KEY);
      }
    }
    if (!api.getPanel(GRAPH_CENTER_PANEL_ID)) {
      api.addPanel({
        id: GRAPH_CENTER_PANEL_ID,
        component: "graph",
        title: "Graph",
        minimumWidth: 320,
      });
    }
    addGraphPanels(api);
    api.onDidLayoutChange(() => saveLayout(WIKI_GRAPH_STORAGE_KEY, api.toJSON()));
  }, []);

  const value: GraphWorkspaceValue = {
    adapted,
    tools,
    dispatchTools,
    selectedId,
    select: setSelectedId,
    path,
    pathFrom,
    pathTo,
    setPathFrom: (id) => {
      setPathFrom(id);
      setPath(null);
    },
    setPathTo: (id) => {
      setPathTo(id);
      setPath(null);
    },
    findPath,
    clearPath: () => setPath(null),
    clusters,
    nodeOptions,
    selectedTitle,
    setViewHandle,
    open,
    fit,
    clear,
    exportSVG,
    exportPNG,
  };

  if (error) return <p className="content__empty">Graph error: {error}</p>;
  if (!data) return <SkeletonLines count={5} label="Loading graph" />;

  if (!DOCKVIEW_ENABLED) {
    return (
      <GraphWorkspaceContext.Provider value={value}>
        <div
          className="dock-layout dock-layout--fallback wiki-graph-fallback"
          data-testid="wiki-graph"
        >
          <section className="dock-content" aria-label="Graph">
            <GraphPanel />
          </section>
          <section className="dock-content" aria-label="Graph controls">
            <GraphControlsPanel />
          </section>
        </div>
      </GraphWorkspaceContext.Provider>
    );
  }

  return (
    <GraphWorkspaceContext.Provider value={value}>
      <DockviewReact
        className="wiki-workspace dock-layout"
        theme={themeCatppuccinMochaSpaced}
        components={components}
        onReady={onReady}
      />
    </GraphWorkspaceContext.Provider>
  );
}
