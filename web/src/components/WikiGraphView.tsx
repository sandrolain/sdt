import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
  type ComponentType,
  type Ref,
} from "react";
import ForceGraph2DBase from "react-force-graph-2d";
import {
  adaptGraph,
  fetchWikiGraph,
  type GLink,
  type GraphData,
  type GNode,
} from "../lib/graphModel";
import { applyLayout } from "../lib/graphLayout";
import { labelObject, labelSpriteSpec } from "../lib/graphSprites";
import {
  computeHighlight,
  initialSelection,
  linkVisual,
  linkWidthFor,
  nodeVisual,
  selectionReducer,
} from "../lib/graphSelection";
import { graphToolsReducer, initialGraphTools, visibleSet } from "../lib/graphTools";
import { GraphToolsPanel } from "./GraphToolsPanel";
import { SkeletonLines } from "./Skeleton";
import { useReloadToken } from "../lib/useReloadToken";
import { useOpenDocs } from "../lib/openDocsContext";

const ForceGraph3D = lazy(() => import("react-force-graph-3d"));

interface GraphHandle {
  zoomToFit?: (durationMs?: number, padding?: number, filter?: (n: GNode) => boolean) => void;
}

/** 3D background: dark enough that the node glow reads without washing the scene. */
const BG = "#0d0d15";

/**
 * Narrow prop contract shared by the 2D and 3D renderers (their generic
 * signatures diverge; the concrete components are cast once, here).
 */
interface ForceGraphViewProps {
  graphData: { nodes: GNode[]; links: GLink[] };
  nodeId?: string;
  nodeVal?: (n: GNode) => number;
  nodeLabel?: (n: GNode) => string;
  nodeColor?: (n: GNode) => string;
  linkColor?: (l: GLink) => string;
  linkWidth?: (l: GLink) => number;
  onNodeClick?: (n: GNode) => void;
  onNodeHover?: (n: GNode | null) => void;
  onBackgroundClick?: () => void;
  backgroundColor?: string;
  nodeCanvasObject?: (n: GNode, ctx: CanvasRenderingContext2D, globalScale: number) => void;
  showNavInfo?: boolean;
  ref?: Ref<GraphHandle | undefined>;
  /** 3D-only: replaces the default sphere with a canvas-sprite per node. */
  nodeThreeObject?: (n: GNode) => unknown;
  nodeThreeObjectExtend?: boolean;
}

const ForceGraph2D = ForceGraph2DBase as unknown as ComponentType<ForceGraphViewProps>;

const LABEL_COLOR = "#cdd6f4";
const EDGE_COLOR = "#6c7086";
const EDGE_ACTIVE = "#cba6f7";
const EDGE_DIM = "#313244";

/** Wiki graph view: 2D/3D react-force-graph modes with a shared tools panel. */
export function WikiGraphView() {
  const [data, setData] = useState<GraphData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tools, dispatchTools] = useReducer(graphToolsReducer, initialGraphTools);
  const [sel, dispatchSel] = useReducer(selectionReducer, initialSelection);
  const graphRef = useRef<GraphHandle | undefined>(undefined);
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
        ? adaptGraph(data, {
            clusterKey: tools.clusterKey,
            visibleVerbs: visibleSet(allVerbs, tools.hiddenVerbs),
            visibleKinds: visibleSet(allKinds, tools.hiddenKinds),
          })
        : null,
    [data, tools.clusterKey, tools.hiddenVerbs, tools.hiddenKinds, allVerbs, allKinds],
  );

  const graphData = useMemo(() => {
    if (!adapted) return { nodes: [], links: [] };
    return { nodes: applyLayout(adapted.nodes, adapted.links, tools.layout), links: adapted.links };
  }, [adapted, tools.layout]);

  const highlight = useMemo(() => computeHighlight(adapted?.links ?? [], sel), [adapted, sel]);

  const clusters = useMemo(() => {
    if (!adapted) return [];
    const counts = new Map<string, number>();
    for (const n of adapted.nodes) counts.set(n.cluster, (counts.get(n.cluster) ?? 0) + 1);
    return [...counts.entries()]
      .map(([id, count]) => ({ id, count, color: adapted.palette.get(id) ?? "#9399b2" }))
      .sort((a, b) => a.id.localeCompare(b.id));
  }, [adapted]);

  const fit = useCallback(() => {
    graphRef.current?.zoomToFit?.(400, 60);
  }, []);

  useEffect(() => {
    if (tools.layout) fit();
  }, [tools.layout, tools.clusterKey, fit]);

  const selectedTitle = useMemo(() => {
    if (!sel.selected || !adapted) return null;
    return adapted.nodes.find((n) => n.id === sel.selected)?.title ?? sel.selected;
  }, [sel.selected, adapted]);

  const focusedId = sel.selected ?? sel.hovered;

  const open = useCallback(
    (id: string) => {
      openDoc(`context/wiki/${id}.md`);
    },
    [openDoc],
  );

  const handleNodeClick = useCallback(
    (node: GNode) => {
      if (sel.selected === node.id) {
        open(node.id);
        return;
      }
      dispatchSel({ type: "select", id: node.id });
      dispatchTools({ type: "focus", value: node.id });
    },
    [sel.selected, open],
  );

  if (error) return <p className="content__empty">Graph error: {error}</p>;
  if (!data || !adapted) return <SkeletonLines count={5} label="Loading graph" />;

  const commonProps: ForceGraphViewProps = {
    graphData,
    nodeId: "id",
    nodeVal: (n: GNode) => (n.val ?? 1) * (nodeVisual(n.id, highlight).emphasis ? 2 : 1),
    nodeLabel: (n: GNode) => n.title,
    nodeColor: (n: GNode) =>
      highlight.active && nodeVisual(n.id, highlight).alpha < 0.5
        ? EDGE_DIM
        : (n.color ?? EDGE_COLOR),
    linkColor: (l: GLink) =>
      highlight.active
        ? linkVisual(l, highlight).alpha > 0.5
          ? EDGE_ACTIVE
          : EDGE_DIM
        : EDGE_COLOR,
    linkWidth: (l: GLink) => linkWidthFor(l, highlight),
    onNodeClick: handleNodeClick,
    onNodeHover: (n: GNode | null) => dispatchSel({ type: "hover", id: n?.id ?? null }),
    onBackgroundClick: () => dispatchSel({ type: "clear" }),
    backgroundColor: BG,
    ref: graphRef,
  };

  const ForceGraph3DLazy = ForceGraph3D as unknown as ComponentType<ForceGraphViewProps>;

  return (
    <div className="graph-view">
      <GraphToolsPanel
        tools={tools}
        allVerbs={allVerbs}
        allKinds={allKinds}
        selectedId={sel.selected}
        focusedId={focusedId}
        selectedTitle={selectedTitle}
        clusters={clusters}
        onTools={dispatchTools}
        onFit={fit}
        onClear={() => {
          dispatchSel({ type: "clear" });
          if (sel.selected) {
            dispatchTools({ type: "focus", value: null });
            fit();
          }
        }}
        onOpen={open}
      />
      <div className="graph-view__canvas">
        {tools.mode === "2d" ? (
          <ForceGraph2D
            {...commonProps}
            nodeCanvasObject={(node, ctx, globalScale) =>
              draw2DNode(node, ctx, globalScale, tools.showLabels, highlight)
            }
          />
        ) : (
          <Suspense fallback={<p className="content__empty">Loading 3D renderer…</p>}>
            <ForceGraph3DLazy
              {...commonProps}
              linkWidth={(l: GLink) => linkWidthFor(l, highlight, 4)}
              nodeThreeObject={(node) =>
                labelObject(labelSpriteSpec(node, nodeVisual(node.id, highlight), tools.showLabels))
              }
              nodeThreeObjectExtend={false}
            />
          </Suspense>
        )}
      </div>
    </div>
  );
}

function draw2DNode(
  node: GNode,
  ctx: CanvasRenderingContext2D,
  globalScale: number,
  showLabels: boolean,
  highlight: ReturnType<typeof computeHighlight>,
): void {
  const visual = nodeVisual(node.id, highlight);
  const radius = Math.sqrt(Math.max(1, node.val ?? 1)) * 2.4;
  ctx.globalAlpha = visual.alpha;
  ctx.beginPath();
  ctx.arc(node.x ?? 0, node.y ?? 0, radius, 0, Math.PI * 2);
  ctx.fillStyle = node.color ?? EDGE_COLOR;
  ctx.fill();
  if (showLabels && (visual.emphasis || !highlight.active)) {
    const fontSize = Math.max(9, 12 / globalScale);
    ctx.font = `${fontSize}px "IBM Plex Sans", sans-serif`;
    ctx.textAlign = "center";
    ctx.fillStyle = LABEL_COLOR;
    ctx.fillText(node.title, node.x ?? 0, (node.y ?? 0) - radius - 2);
  }
  ctx.globalAlpha = 1;
}
