/**
 * GraphWorkspace — the shared dockview graph surface.
 *
 * Mounts a graph-scoped `DockviewReact` (centre = `GraphView`, right edge =
 * `GraphControls`, plus the node-detail panel) over an already-adapted engine
 * graph. The data source, the node-open behaviour and the persisted layout key
 * are props, so the wiki graph (`/wiki/graph`) and the semantic map
 * (`/docs/map`) render the same component over different data.
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
  DockviewDefaultTab,
  DockviewReact,
  type IDockviewPanelHeaderProps,
  type IDockviewPanelProps,
} from "dockview-react";
import type { AdaptedEngineGraph } from "../lib/graph/adapter";
import type { ClusterKey } from "../lib/graphModel";
import type { GraphLayout } from "../lib/graph/types";
import {
  adaptSemanticGraph,
  semanticModeProjects,
  type SemanticGraph,
  type SemanticMode,
} from "../lib/semantic";
import { findShortestPath, type ShortestPath } from "../lib/graph/path";
import { edgeHoverInfo } from "../lib/graph/tooltip";
import type { GraphLinkInput } from "../lib/graph/types";
import {
  graphToolsReducer,
  initialGraphTools,
  type GraphToolsAction,
  type GraphToolsState,
} from "../lib/graphTools";
import { useDockviewTheme, useDockviewWorkspace } from "../lib/dockviewWorkspace";
import { addGraphPanels, GRAPH_CENTER_PANEL_ID } from "../lib/wikiEdgeLayout";
import { GraphView, type GraphViewHandle } from "./GraphView";
import { GraphControls } from "./GraphControls";
import { GraphNodeDetail } from "./GraphNodeDetail";
import { graphNeighbours } from "../lib/graph/neighbours";
import { SkeletonLines } from "./Skeleton";

interface GraphWorkspaceValue {
  adapted: AdaptedEngineGraph;
  tools: GraphToolsState;
  dispatchTools: Dispatch<GraphToolsAction>;
  /** The layout the view renders: `projection` when the semantic mode projects. */
  viewLayout: GraphLayout;
  /** The docs-map semantic mode, or null on a surface without the control. */
  semanticMode: SemanticMode | null;
  setSemanticMode: (mode: SemanticMode) => void;
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
  if (!value) throw new Error("useGraphWorkspace must be used inside GraphWorkspace");
  return value;
}

function GraphPanel() {
  const { setViewHandle, adapted, tools, viewLayout, selectedId, select, path, open } =
    useGraphWorkspace();
  const [hoverEdge, setHoverEdge] = useState<GraphLinkInput | null>(null);
  const labels = useMemo(
    () => new Map(adapted.nodes.map((n) => [n.id, String(n.label ?? n.id)])),
    [adapted],
  );
  const labelOf = useCallback((id: string) => labels.get(id) ?? id, [labels]);
  const info = hoverEdge ? edgeHoverInfo(hoverEdge, labelOf) : null;
  return (
    <div className="graph-panel">
      <GraphView
        ref={setViewHandle}
        nodes={adapted.nodes}
        links={adapted.links}
        mode={tools.mode}
        layout={viewLayout}
        selectedId={selectedId}
        onSelect={select}
        highlightPath={path}
        labels={tools.showLabels ? "auto" : "none"}
        hiddenGroups={tools.hiddenGroups}
        hiddenRelations={tools.hiddenRelations}
        hiddenKinds={tools.hiddenKinds}
        centrality={tools.centrality}
        neighborsOnly={tools.neighborsOnly}
        onEdgeHover={setHoverEdge}
        onNodeDoubleClick={(node) => open(String(node.path ?? node.id))}
      />
      {info ? (
        <aside className="graph-edge-tooltip" role="status" aria-live="polite">
          <span className="graph-edge-tooltip__verb">{info.verb}</span>
          <span className="graph-edge-tooltip__ends">
            {info.fromLabel} → {info.toLabel}
          </span>
          {info.label ? <span className="graph-edge-tooltip__label">{info.label}</span> : null}
        </aside>
      ) : null}
    </div>
  );
}

function GraphControlsPanel() {
  const g = useGraphWorkspace();
  const semantic =
    g.semanticMode != null ? { mode: g.semanticMode, onChange: g.setSemanticMode } : undefined;
  return (
    <div className="dock-content graph-controls-panel">
      <GraphControls
        tools={g.tools}
        allVerbs={g.adapted.allVerbs}
        allKinds={g.adapted.allKinds}
        clusters={g.clusters}
        selectedId={g.selectedId}
        nodeOptions={g.nodeOptions}
        pathFrom={g.pathFrom}
        pathTo={g.pathTo}
        pathActive={g.path !== null}
        semantic={semantic}
        onTools={g.dispatchTools}
        onPathFrom={g.setPathFrom}
        onPathTo={g.setPathTo}
        onFindPath={g.findPath}
        onClearPath={g.clearPath}
        onFit={g.fit}
        onClear={g.clear}
        onExportSVG={g.exportSVG}
        onExportPNG={g.exportPNG}
      />
    </div>
  );
}

/** The right-hand node detail panel (B3): always present, explicit empty state. */
function GraphDetailPanel() {
  const { adapted, selectedId, select, open } = useGraphWorkspace();
  const node = useMemo(
    () => (selectedId ? (adapted.nodes.find((n) => n.id === selectedId) ?? null) : null),
    [adapted, selectedId],
  );
  const neighbours = useMemo(() => graphNeighbours(adapted, selectedId), [adapted, selectedId]);
  return (
    <div className="dock-content graph-detail-panel">
      <GraphNodeDetail node={node} neighbours={neighbours} onSelect={select} onOpen={open} />
    </div>
  );
}

/** Stable dockview component registry (module scope: never recreated). */
const components: Record<string, (props: IDockviewPanelProps) => ReactNode> = {
  graph: () => <GraphPanel />,
  "graph-controls": () => <GraphControlsPanel />,
  "graph-detail": () => <GraphDetailPanel />,
};

/**
 * Graph tabs render the title only: no close action exists on the graph
 * surface (B1), while the edge group keeps its collapse chevron. The action is
 * never created, so the guarantee does not rest on CSS.
 */
function GraphTab(props: IDockviewPanelHeaderProps) {
  return <DockviewDefaultTab {...props} hideClose />;
}

export interface GraphWorkspaceProps {
  /** The already-adapted engine graph to render. */
  adapted: AdaptedEngineGraph;
  /** Re-adapt the graph when the cluster key changes (wiki graph only). */
  recluster?: (key: ClusterKey) => AdaptedEngineGraph;
  /** Persisted dockview layout key (one per data source). */
  storageKey: string;
  /** The docs semantic-map payload. When present it enables the Semantic-mode
   * control and its mode-aware adaptation supersedes `adapted`. */
  semanticGraph?: SemanticGraph;
  /** Open a node's document (wiki page or corpus path). */
  open: (id: string) => void;
  loading: boolean;
  error: string | null;
  /** Centre panel title, e.g. "Graph" or "Map". */
  title: string;
}

export function GraphWorkspace({
  adapted,
  recluster,
  storageKey,
  semanticGraph,
  open: openProp,
  loading,
  error,
  title,
}: GraphWorkspaceProps) {
  const [tools, dispatchTools] = useReducer(graphToolsReducer, initialGraphTools);
  const graph = useMemo(
    () =>
      semanticGraph
        ? adaptSemanticGraph(semanticGraph, tools.semanticMode)
        : (recluster?.(tools.clusterKey) ?? adapted),
    [semanticGraph, tools.semanticMode, recluster, tools.clusterKey, adapted],
  );
  const viewLayout: GraphLayout =
    semanticGraph && semanticModeProjects(tools.semanticMode) ? "projection" : tools.layout;
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [path, setPath] = useState<ShortestPath | null>(null);
  const [pathFrom, setPathFrom] = useState("");
  const [pathTo, setPathTo] = useState("");
  const viewRef = useRef<GraphViewHandle | null>(null);
  const setViewHandle = useCallback((handle: GraphViewHandle | null) => {
    viewRef.current = handle;
  }, []);
  const dockviewTheme = useDockviewTheme();
  const { enabled: dockviewEnabled, onReady } = useDockviewWorkspace(storageKey, {
    ensureCenter: (api) => {
      if (!api.getPanel(GRAPH_CENTER_PANEL_ID)) {
        api.addPanel({
          id: GRAPH_CENTER_PANEL_ID,
          component: "graph",
          title,
          minimumWidth: 320,
        });
      }
    },
    addPanels: addGraphPanels,
  });

  const clusters = useMemo(() => {
    const counts = new Map<string, number>();
    for (const n of graph.nodes)
      counts.set(String(n.group), (counts.get(String(n.group)) ?? 0) + 1);
    return [...counts.entries()]
      .map(([id, count]) => ({ id, count, color: graph.palette.get(id) ?? "#9399b2" }))
      .sort((a, b) => a.id.localeCompare(b.id));
  }, [graph]);

  const nodeOptions = useMemo(
    () =>
      graph.nodes
        .map((n) => ({ id: n.id, label: String(n.label ?? n.id) }))
        .sort((a, b) => a.label.localeCompare(b.label)),
    [graph],
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
    const shortest = findShortestPath(pathFrom, pathTo, graph.nodes, graph.links);
    setPath(shortest);
    if (shortest) viewRef.current?.focusNode(pathFrom);
  }, [pathFrom, pathTo, graph]);

  useEffect(() => {
    if (viewLayout) fit();
  }, [viewLayout, tools.clusterKey, fit]);

  const value: GraphWorkspaceValue = {
    adapted: graph,
    tools,
    dispatchTools,
    viewLayout,
    semanticMode: semanticGraph ? tools.semanticMode : null,
    setSemanticMode: (mode) => dispatchTools({ type: "semanticMode", value: mode }),
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
    setViewHandle,
    open: openProp,
    fit,
    clear,
    exportSVG,
    exportPNG,
  };

  if (error)
    return (
      <p className="content__empty">
        {title} error: {error}
      </p>
    );
  if (loading) return <SkeletonLines count={5} label={`Loading ${title.toLowerCase()}`} />;

  if (!dockviewEnabled) {
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
          <section className="dock-content" aria-label="Graph detail">
            <GraphDetailPanel />
          </section>
        </div>
      </GraphWorkspaceContext.Provider>
    );
  }

  return (
    <GraphWorkspaceContext.Provider value={value}>
      <DockviewReact
        className="wiki-graph-workspace wiki-workspace dock-layout"
        theme={dockviewTheme}
        components={components}
        defaultTabComponent={GraphTab}
        onReady={onReady}
      />
    </GraphWorkspaceContext.Provider>
  );
}
