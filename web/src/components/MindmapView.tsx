import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  type ReactFlowProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { annotateBoundaries, annotateSummaries, boundaryRects } from "../lib/boundaries";
import {
  DEFAULT_FUSE_OPTIONS,
  extractMapRefs,
  fuseMap,
  type FuseStats,
  type MapRef,
} from "../lib/fuse";
import { annotateGroups } from "../lib/groups";
import { groupHulls } from "../lib/groupHull";
import { downloadBlob, mapToSvg, svgToPng } from "../lib/mapExport";
import { layoutMap, MAP_LAYOUTS, type MapLayoutKind } from "../lib/mapLayout";
import {
  buildMapGraph,
  nodeRects,
  visibleNodes,
  type MapFlowEdge,
  type MapFlowNode,
} from "../lib/mapModel";
import { measureTree, type MeasuredNode } from "../lib/mapMetrics";
import { parseMapDocument, type MapNodeKind, type MindNode } from "../lib/mindmap";
import { fetchDoc, isCanvas, isMermaid } from "../lib/api";
import { isMapPath } from "../lib/documentModes";
import { Icon } from "../lib/icon";
import { loadMapIndex, loadWikiIndex, type MapIndexEntry } from "../lib/wikiIndexLoader";
import type { WikiIndex } from "../lib/wikiLinks";
import { MapNodeView } from "./MindMapNode";
import { MapOverlay } from "./MapOverlay";

interface MindmapViewProps {
  markdown: string;
  basePath: string;
  title: string;
}

/** Stable node-type registry (module scope: React Flow warns on identity churn). */
const mapNodeTypes = { map: MapNodeView };

/** Zoom controls wired to the React Flow viewport (must sit inside the provider). */
function MapZoom() {
  const { zoomIn, zoomOut, fitView } = useReactFlow();
  return (
    <div className="mindmap__controls" role="group" aria-label="Map zoom">
      <button type="button" className="graph-tools__button" onClick={() => zoomIn()}>
        Zoom in
      </button>
      <button type="button" className="graph-tools__button" onClick={() => zoomOut()}>
        Zoom out
      </button>
      <button type="button" className="graph-tools__button" onClick={() => fitView()}>
        Fit
      </button>
    </div>
  );
}

/**
 * Re-fit when the layout changes: the two layouts have very different extents,
 * so keeping the old viewport would leave half the map off-screen. The first
 * fit is React Flow's own `fitView` prop, so it is skipped here.
 */
function FitOnChange({ token }: { token: string }) {
  const { fitView } = useReactFlow();
  const first = useRef(true);
  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    const frame = requestAnimationFrame(() => void fitView({ padding: 0.08, duration: 200 }));
    return () => cancelAnimationFrame(frame);
  }, [token, fitView]);
  return null;
}

const FLOW_PROPS: Partial<ReactFlowProps> = {
  minZoom: 0.1,
  maxZoom: 4,
  nodesDraggable: false,
  nodesConnectable: false,
  elementsSelectable: true,
  panOnDrag: true,
  zoomOnScroll: true,
  fitView: true,
  proOptions: { hideAttribution: true },
};

/** React Flow mind map with the XMindMark boundary, summary and group overlays. */
export function MindmapView({ markdown, basePath, title }: MindmapViewProps) {
  const [wikiIndex, setWikiIndex] = useState<WikiIndex | undefined>(undefined);
  const [mapIndex, setMapIndex] = useState<Map<string, MapIndexEntry>>(new Map());
  const [mode, setMode] = useState<"current" | "fused">("current");
  const [layout, setLayout] = useState<MapLayoutKind>("balanced");
  const [fused, setFused] = useState<{ path: string; root: MindNode; stats: FuseStats } | null>(
    null,
  );
  const [refError, setRefError] = useState<string | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    loadWikiIndex()
      .then((ix) => alive && setWikiIndex(ix))
      .catch(() => undefined);
    loadMapIndex()
      .then((mi) => alive && setMapIndex(mi))
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  const isMap = isMapPath(basePath);
  const currentRoot = useMemo(
    () => parseMapDocument(markdown, { basePath, wikiIndex }),
    [markdown, basePath, wikiIndex],
  );

  const referencedIds = useMemo(
    () => extractMapRefs(currentRoot, mapIndex),
    [currentRoot, mapIndex],
  );

  useEffect(() => {
    if (mode !== "fused" || !isMap) return;
    let alive = true;
    const refs = new Map<string, MapRef>();
    const load = async () => {
      for (const id of referencedIds) {
        const entry = mapIndex.get(id);
        if (!entry) continue;
        const doc = await fetchDoc(entry.path);
        if (isCanvas(doc) || isMermaid(doc)) continue;
        refs.set(entry.id, {
          id: entry.id,
          title: entry.title,
          root: parseMapDocument(doc.markdown, { basePath: entry.path, wikiIndex }),
        });
      }
      if (!alive) return;
      const result = fuseMap(basePath, currentRoot, refs, DEFAULT_FUSE_OPTIONS);
      setFused({ path: basePath, root: result.root, stats: result.stats });
      setRefError(null);
    };
    load().catch((err: unknown) => {
      if (alive) setRefError(err instanceof Error ? err.message : String(err));
    });
    return () => {
      alive = false;
    };
  }, [mode, isMap, referencedIds, mapIndex, currentRoot, basePath, wikiIndex]);

  const current = mode === "fused" && fused?.path === basePath && fused ? fused : null;
  const activeRoot = current ? current.root : currentRoot;
  const fusedStats = current?.stats ?? null;

  const tree: MeasuredNode = useMemo(() => measureTree(activeRoot), [activeRoot]);

  // A document starts from the folded state it declares (`[F]`), and a new
  // document (or a new fused map) forgets the reader's toggles: the state is
  // adjusted during render rather than from an effect.
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(() => foldedIds(tree));
  const docKey = `${basePath}|${mode}|${markdown}`;
  const [expandedFor, setExpandedFor] = useState(docKey);
  if (expandedFor !== docKey) {
    setExpandedFor(docKey);
    setCollapsed(foldedIds(tree));
  }
  const visible = useMemo(() => visibleNodes(tree, collapsed), [tree, collapsed]);

  // A node whose whole label is one link opens its target: the label is already
  // an anchor in the label HTML, so the node itself is a button.
  const navigate = useNavigate();
  const open = useCallback((href: string) => navigate(href), [navigate]);

  const toggle = useCallback((id: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const { positions, bounds } = layoutMap(tree, layout);
  const { nodes, edges } = useMemo(
    () => buildMapGraph(tree, positions, { collapsed, onToggle: toggle, onOpen: open }),
    [tree, positions, collapsed, toggle, open],
  );
  const rects = useMemo(() => nodeRects(nodes), [nodes]);
  const overlay = useMemo(() => {
    const titles = activeRoot.payload?.titles;
    const pruned = pruneToVisible(tree, visible);
    const padding = 10;
    return {
      boundaries: boundaryRects(annotateBoundaries(pruned, titles), rects, padding),
      summaries: boundaryRects(annotateSummaries(pruned, titles), rects, padding),
      hulls: groupHulls(annotateGroups(pruned), rects),
    };
  }, [tree, visible, rects, activeRoot]);

  const exportName = basePath.split("/").pop() ?? "map";
  const exportSvg = useCallback(() => {
    const svg = mapToSvg({ nodes, edges, ...overlay, bounds, title });
    downloadBlob(new Blob([svg], { type: "image/svg+xml" }), `${exportName}.svg`);
  }, [nodes, edges, overlay, bounds, title, exportName]);
  const exportPng = useCallback(async () => {
    try {
      const svg = mapToSvg({ nodes, edges, ...overlay, bounds, title });
      downloadBlob(await svgToPng(svg), `${exportName}.png`);
      setExportError(null);
    } catch (err: unknown) {
      setExportError(err instanceof Error ? err.message : String(err));
    }
  }, [nodes, edges, overlay, bounds, title, exportName]);

  return (
    <div className="mindmap">
      <div className="mindmap__toolbar">
        <span className="mindmap__title">Mindmap</span>
        <div className="graph-tools__row" role="group" aria-label="Map layout">
          {MAP_LAYOUTS.map((option) => (
            <button
              key={option.id}
              type="button"
              className={`graph-tools__chip${layout === option.id ? " is-active" : ""}`}
              aria-pressed={layout === option.id}
              onClick={() => setLayout(option.id)}
            >
              <Icon name={option.id === "balanced" ? "call_split" : "radio_button_unchecked"} />
              {option.label}
            </button>
          ))}
        </div>
        {isMap && referencedIds.length > 0 && (
          <div className="graph-tools__row" role="group" aria-label="Map mode">
            <button
              type="button"
              className={`graph-tools__chip${mode === "current" ? " is-active" : ""}`}
              aria-pressed={mode === "current"}
              onClick={() => setMode("current")}
            >
              <Icon name="my_location" />
              Current Map
            </button>
            <button
              type="button"
              className={`graph-tools__chip${mode === "fused" ? " is-active" : ""}`}
              aria-pressed={mode === "fused"}
              onClick={() => setMode("fused")}
            >
              <Icon name="merge" />
              Fused Map
            </button>
          </div>
        )}
        <div className="graph-tools__row" role="group" aria-label="Map export">
          <button type="button" className="graph-tools__chip" onClick={exportSvg}>
            <Icon name="download" />
            SVG
          </button>
          <button type="button" className="graph-tools__chip" onClick={() => void exportPng()}>
            <Icon name="image" />
            PNG
          </button>
        </div>
        {mode === "fused" && fusedStats && (
          <span className="mindmap__stats">
            {fusedStats.imported.length} imported
            {fusedStats.deduped.length > 0 ? `, ${fusedStats.deduped.length} deduped` : ""}
            {fusedStats.cycles.length > 0 ? `, ${fusedStats.cycles.length} cycles skipped` : ""}
          </span>
        )}
      </div>
      {refError && <p className="content__empty">Fused map error: {refError}</p>}
      {exportError && <p className="content__empty">Export failed: {exportError}</p>}
      <ReactFlowProvider>
        <MapZoom />
        <FitOnChange token={layout} />
        <div
          className="mindmap__viewport"
          role="application"
          aria-label={`Mind map: ${title} (read-only)`}
        >
          <MapOverlay {...overlay} />
          <ReactFlow
            nodes={nodes as MapFlowNode[]}
            edges={edges as MapFlowEdge[]}
            nodeTypes={mapNodeTypes}
            {...FLOW_PROPS}
          >
            <Background />
            <Controls showInteractive={false} />
            {/* bgColor/maskColor are inline styles on React Flow's minimap, so
                the tokens have to travel with the props, not with CSS. */}
            <MiniMap
              pannable
              zoomable
              bgColor="var(--bg-mantle)"
              maskColor="var(--bg-base)"
              nodeColor={() => "#6c7086"}
            />
          </ReactFlow>
        </div>
      </ReactFlowProvider>
    </div>
  );
}

/** Ids of the nodes the document marks folded (`[F]`). */
function foldedIds(tree: MeasuredNode): Set<string> {
  const out = new Set<string>();
  const visit = (node: MeasuredNode) => {
    if (node.markers.folded) out.add(node.id);
    node.children.forEach(visit);
  };
  visit(tree);
  return out;
}

/** The measured tree with collapsed subtrees detached, for the overlays. */
function pruneToVisible(tree: MeasuredNode, visible: MeasuredNode[]): MeasuredNode {
  const keep = new Set(visible.map((n) => n.id));
  const prune = (node: MeasuredNode): MeasuredNode => ({
    ...node,
    children: node.children.filter((c) => keep.has(c.id)).map(prune),
  });
  return prune(tree);
}

/** Node kind re-exported for the exporter and the model tests. */
export type { MapNodeKind };
