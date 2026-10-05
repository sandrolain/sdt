import type { Edge, Node } from "@xyflow/react";
import { edgeSides, type MapLayoutKind, type MapPoint } from "./mapLayout";
import type { MeasuredNode } from "./mapMetrics";
import type { MapNodeKind } from "./mindmap";

/**
 * The measured tree as a React Flow graph: nodes carry the label, the node
 * kind, the markers and the callbacks; edges are the parent→child links plus
 * one labelled edge per paired `[n]`/`[^n]` relationship. A collapsed node
 * removes its descendants, so the graph always describes the visible map and
 * the layout only ever sees visible nodes.
 */

/** Data carried by every map node. */
export interface MapNodeData extends Record<string, unknown> {
  /** label HTML, sanitised at render time */
  content: string;
  /** label text, for an accessible name and the exporter */
  text: string;
  kind: MapNodeKind;
  depth: number;
  notes: string[];
  /** `[L:url]` */
  link?: string;
  stickers: string[];
  groups: string[];
  /** the target when the whole label is a single link, so the node opens it */
  href?: string;
  hasChildren: boolean;
  collapsed: boolean;
  /** `[F]`: the node starts folded */
  folded: boolean;
  onToggle?: (id: string) => void;
  onOpen?: (href: string) => void;
}

export type MapFlowNode = Node<MapNodeData>;
export type MapFlowEdge = Edge;

export interface NodeRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface MapGraph {
  nodes: MapFlowNode[];
  edges: MapFlowEdge[];
}

export interface BuildOptions {
  collapsed?: ReadonlySet<string>;
  onToggle?: (id: string) => void;
  onOpen?: (href: string) => void;
  /** Layout that produced `positions`, so parent edges carry the right sides. */
  layout?: MapLayoutKind;
}

// Categorical identity in the graph palette: like the board edges, these are
// canvas-drawn graph colours and stay raw values.
const EDGE_COLOR = "#6c7086";
const RELATION_COLOR = "#f38ba8";

const SINGLE_LINK_RE = /^<a href="([^"]+)">([\s\S]*)<\/a>$/;

/** The nodes the map shows: a collapsed node hides everything under it. */
export function visibleNodes(tree: MeasuredNode, collapsed: ReadonlySet<string>): MeasuredNode[] {
  const out: MeasuredNode[] = [];
  const visit = (node: MeasuredNode, hidden: boolean) => {
    if (!hidden) out.push(node);
    const hideChildren = hidden || collapsed.has(node.id);
    for (const child of node.children) visit(child, hideChildren);
  };
  visit(tree, false);
  return out;
}

/** The whole label is one link → the node itself opens it (click-to-open). */
function soleHref(content: string, links: string[]): string | undefined {
  if (links.length !== 1) return undefined;
  return SINGLE_LINK_RE.exec(content)?.[1];
}

/** Build the read-only React Flow graph for a measured tree. */
export function buildMapGraph(
  tree: MeasuredNode,
  positions: Map<string, MapPoint>,
  options: BuildOptions = {},
): MapGraph {
  const collapsed = options.collapsed ?? new Set<string>();
  const visible = visibleNodes(tree, collapsed);
  const nodes: MapFlowNode[] = visible.map((node) => {
    const point = positions.get(node.id) ?? { x: 0, y: 0 };
    const data: MapNodeData = {
      content: node.content,
      text: node.lines.join(" "),
      kind: node.kind,
      depth: node.depth,
      notes: node.markers.notes,
      link: node.markers.link,
      stickers: node.markers.stickers,
      groups: node.markers.groups,
      href: soleHref(node.content, node.links),
      hasChildren: node.children.length > 0,
      collapsed: collapsed.has(node.id),
      folded: node.markers.folded,
      onToggle: options.onToggle,
      onOpen: options.onOpen,
    };
    return {
      id: node.id,
      type: "map",
      position: { x: point.x - node.box.width / 2, y: point.y - node.box.height / 2 },
      data,
      initialWidth: node.box.width,
      initialHeight: node.box.height,
      style: { width: node.box.width, height: node.box.height },
      draggable: false,
      connectable: false,
      selectable: true,
      zIndex: 1,
    };
  });
  return { nodes, edges: buildEdges(visible, collapsed, positions, options.layout ?? "balanced") };
}

/** Parent→child edges plus one edge per paired relationship. */
function buildEdges(
  visible: MeasuredNode[],
  collapsed: ReadonlySet<string>,
  positions: Map<string, MapPoint>,
  layout: MapLayoutKind,
): MapFlowEdge[] {
  const byId = new Map(visible.map((n) => [n.id, n]));
  const edges: MapFlowEdge[] = [];
  for (const node of visible) {
    if (!node.parent) continue;
    if (!byId.has(node.parent)) continue;
    if (collapsed.has(node.parent)) continue;
    const parentPoint = positions.get(node.parent);
    const childPoint = positions.get(node.id);
    const sides =
      parentPoint && childPoint ? edgeSides(layout, parentPoint, childPoint) : undefined;
    edges.push({
      id: `e:${node.parent}->${node.id}`,
      source: node.parent,
      target: node.id,
      type: "default",
      data: sides ? { fromSide: sides[0], toSide: sides[1] } : undefined,
      style: { stroke: EDGE_COLOR, strokeWidth: 1.5 },
    });
  }
  const sources = new Map<string, MeasuredNode>();
  const targets = new Map<string, MeasuredNode>();
  for (const node of visible) {
    const relation = node.markers.relation;
    if (!relation) continue;
    if (relation.direction === "source") sources.set(relation.id, node);
    else targets.set(relation.id, node);
  }
  for (const [id, source] of sources) {
    const target = targets.get(id);
    if (!target) continue;
    edges.push({
      id: `r:${id}`,
      source: source.id,
      target: target.id,
      type: "default",
      label: source.markers.relation?.title ?? target.markers.relation?.title,
      labelStyle: { fill: RELATION_COLOR, fontSize: 11 },
      style: { stroke: RELATION_COLOR, strokeWidth: 1.5, strokeDasharray: "5 4" },
      zIndex: 0,
    });
  }
  return edges;
}

/** Node rectangles in layout coordinates, for the boundary/hull overlays. */
export function nodeRects(nodes: MapFlowNode[]): Map<string, NodeRect> {
  const rects = new Map<string, NodeRect>();
  for (const node of nodes) {
    rects.set(node.id, {
      x: node.position.x,
      y: node.position.y,
      width: node.initialWidth ?? 0,
      height: node.initialHeight ?? 0,
    });
  }
  return rects;
}
