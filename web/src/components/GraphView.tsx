/**
 * GraphView — the React view over the ported three.js engine.
 *
 * Owns the host div, the engine lifecycle and the event bridge; it exposes a
 * controlled `mode`/`layout`/`selectedId` contract plus an imperative ref, and
 * renders **no** controls, legend or detail (analysis D2). Those live in
 * `GraphControls` inside the docked sidebar.
 */
import { forwardRef, useEffect, useImperativeHandle, useMemo, useRef } from "react";
import { GraphEngine, type EngineColors } from "../lib/graph/engine";
import { GRAPH_BACKDROP_CSS } from "../lib/graph/theme";
import type {
  GraphLayout,
  GraphLinkInput,
  GraphMode,
  GraphNodeInput,
  LabelMode,
} from "../lib/graph/types";

export interface GraphViewHandle {
  focusNode: (id: string) => void;
  fitView: () => void;
  exportSVG: () => void;
  exportPNG: () => void;
}

export interface GraphPath {
  nodeIds: string[];
  links: GraphLinkInput[];
}

export interface GraphViewProps {
  nodes: GraphNodeInput[];
  links: GraphLinkInput[];
  mode: GraphMode;
  layout: GraphLayout;
  selectedId: string | null;
  onSelect: (id: string | null) => void;
  highlightPath?: GraphPath | null;
  hiddenGroups?: string[];
  hiddenRelations?: string[];
  hiddenKinds?: string[];
  labels?: LabelMode;
  nodeScale?: number;
  centrality?: boolean;
  neighborsOnly?: boolean;
  colors?: EngineColors;
  background?: string;
  onNodeClick?: (node: GraphNodeInput) => void;
  onNodeDoubleClick?: (node: GraphNodeInput) => void;
  onNodeHover?: (node: GraphNodeInput | null) => void;
  onEdgeHover?: (link: GraphLinkInput | null) => void;
  onBackgroundClick?: () => void;
  className?: string;
}

function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export const GraphView = forwardRef<GraphViewHandle, GraphViewProps>(function GraphView(
  {
    nodes,
    links,
    mode,
    layout,
    selectedId,
    onSelect,
    highlightPath = null,
    hiddenGroups = [],
    hiddenRelations = [],
    hiddenKinds = [],
    labels = "auto",
    nodeScale = 1,
    centrality = false,
    neighborsOnly = false,
    colors,
    background = GRAPH_BACKDROP_CSS,
    onNodeClick,
    onNodeDoubleClick,
    onNodeHover,
    onEdgeHover,
    onBackgroundClick,
    className,
  },
  ref,
) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const engineRef = useRef<GraphEngine | null>(null);

  // Always-fresh callbacks without recreating the engine.
  const cb = useRef({
    onSelect,
    selectedId,
    onNodeClick,
    onNodeDoubleClick,
    onNodeHover,
    onEdgeHover,
    onBackgroundClick,
  });
  useEffect(() => {
    cb.current = {
      onSelect,
      selectedId,
      onNodeClick,
      onNodeDoubleClick,
      onNodeHover,
      onEdgeHover,
      onBackgroundClick,
    };
  });

  useEffect(() => {
    const engine = new GraphEngine(hostRef.current as HTMLDivElement);
    engineRef.current = engine;
    engine.events = {
      onHover: (raw) => cb.current.onNodeHover?.(raw),
      onEdgeHover: (raw) => cb.current.onEdgeHover?.(raw),
      onClick: (raw) => {
        const c = cb.current;
        if (raw) {
          c.onNodeClick?.(raw);
          if (raw.id === c.selectedId) c.onNodeDoubleClick?.(raw);
          else c.onSelect(raw.id);
        } else {
          c.onSelect(null);
          c.onBackgroundClick?.();
        }
      },
      onDoubleClick: (raw) => {
        if (raw) cb.current.onNodeDoubleClick?.(raw);
      },
    };
    return () => {
      engine.dispose();
      engineRef.current = null;
    };
  }, []);

  // Order matters: state and colours before the data (reference 1645-1657).
  useEffect(() => {
    engineRef.current?.setColors(colors ?? {});
  }, [colors]);
  useEffect(() => {
    engineRef.current?.setMode(mode);
  }, [mode]);
  useEffect(() => {
    engineRef.current?.setLayout(layout);
  }, [layout]);
  useEffect(() => {
    engineRef.current?.setSelected(selectedId);
  }, [selectedId]);
  useEffect(() => {
    engineRef.current?.setStyle({ labels, nodeScale });
  }, [labels, nodeScale]);
  useEffect(() => {
    engineRef.current?.setData(nodes, links);
  }, [nodes, links]);
  useEffect(() => {
    engineRef.current?.setPath(highlightPath);
  }, [highlightPath]);
  useEffect(() => {
    engineRef.current?.setFilters(hiddenGroups, hiddenRelations, hiddenKinds);
  }, [hiddenGroups, hiddenRelations, hiddenKinds]);
  useEffect(() => {
    engineRef.current?.setCentrality(centrality);
  }, [centrality]);
  useEffect(() => {
    engineRef.current?.setNeighborhood(neighborsOnly ? selectedId : null);
  }, [neighborsOnly, selectedId]);

  useImperativeHandle(
    ref,
    () => ({
      focusNode: (id) => engineRef.current?.focusNode(id),
      fitView: () => engineRef.current?.fitView(),
      exportSVG: () => {
        const svg = engineRef.current?.toSVG();
        if (svg)
          downloadBlob(
            new Blob([svg], { type: "image/svg+xml;charset=utf-8" }),
            "knowledge-graph.svg",
          );
      },
      exportPNG: () => {
        const engine = engineRef.current;
        const svg = engine?.toSVG();
        if (!engine || !svg) return;
        const url = URL.createObjectURL(new Blob([svg], { type: "image/svg+xml;charset=utf-8" }));
        const image = new Image();
        image.onload = () => {
          const scale = 2;
          const canvas = document.createElement("canvas");
          canvas.width = engine.w * scale;
          canvas.height = engine.h * scale;
          const context = canvas.getContext("2d");
          if (!context) {
            URL.revokeObjectURL(url);
            return;
          }
          context.setTransform(scale, 0, 0, scale, 0, 0);
          context.drawImage(image, 0, 0, engine.w, engine.h);
          canvas.toBlob((blob) => {
            if (blob) downloadBlob(blob, "knowledge-graph.png");
            URL.revokeObjectURL(url);
          }, "image/png");
        };
        image.onerror = () => URL.revokeObjectURL(url);
        image.src = url;
      },
    }),
    [],
  );

  const style = useMemo(
    () => ({
      position: "relative" as const,
      width: "100%",
      height: "100%",
      minHeight: 240,
      overflow: "hidden",
      background,
    }),
    [background],
  );

  return <div ref={hostRef} className={className} style={style} />;
});
