import { edgeSides, type MapLayoutKind, type MapPoint } from "./mapLayout";
import type { MeasuredNode } from "./mapMetrics";
import type { MapNodeKind } from "./mindmap";
import type { CanvasDocument, CanvasEdge, CanvasNode } from "./jsoncanvas/document";

/**
 * The measured tree as a JSON Canvas document (decision 0025): each node
 * carries the label HTML and its map payload under `x-map`, and edges are the
 * parent→child links (with layout-emitted sides) plus one labelled edge per
 * paired `[n]`/`[^n]` relationship. A collapsed node removes its descendants, so
 * the document always describes the visible map and the layout only ever sees
 * visible nodes.
 */

/** Data carried by every map node (the `x-map` payload the body consumes). */
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

/** One visible map node: its box (top-left) and its payload. */
export interface MapNode {
  id: string;
  position: MapPoint;
  width: number;
  height: number;
  data: MapNodeData;
}

export interface NodeRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface MapGraph {
  nodes: MapNode[];
  edges: CanvasEdge[];
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

/** Build the read-only map graph for a measured tree and its layout. */
export function buildMapGraph(
  tree: MeasuredNode,
  positions: Map<string, MapPoint>,
  options: BuildOptions = {},
): MapGraph {
  const collapsed = options.collapsed ?? new Set<string>();
  const visible = visibleNodes(tree, collapsed);
  const nodes: MapNode[] = visible.map((node) => {
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
      position: { x: point.x - node.box.width / 2, y: point.y - node.box.height / 2 },
      width: node.box.width,
      height: node.box.height,
      data,
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
): CanvasEdge[] {
  const byId = new Map(visible.map((n) => [n.id, n]));
  const edges: CanvasEdge[] = [];
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
      fromNode: node.parent,
      toNode: node.id,
      fromSide: sides?.[0],
      toSide: sides?.[1],
      color: EDGE_COLOR,
      "x-kind": "parent",
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
      fromNode: source.id,
      toNode: target.id,
      label: source.markers.relation?.title ?? target.markers.relation?.title,
      color: RELATION_COLOR,
      "x-dash": "5 4",
      "x-kind": "relation",
    });
  }
  return edges;
}

/** Node rectangles in layout coordinates, for the boundary/hull overlays. */
export function nodeRects(nodes: MapNode[]): Map<string, NodeRect> {
  const rects = new Map<string, NodeRect>();
  for (const node of nodes) {
    rects.set(node.id, {
      x: node.position.x,
      y: node.position.y,
      width: node.width,
      height: node.height,
    });
  }
  return rects;
}

/** The map graph as a shared JSON Canvas document for the `JsonCanvas` view. */
export function toCanvasDocument(graph: MapGraph): CanvasDocument {
  // Depth becomes the 3D layer: `x-layer` per node, `x-layers` names each column
  // (the reference's non-standard property the layered view reads).
  const depths = [...new Set(graph.nodes.map((n) => n.data.depth))].sort((a, b) => a - b);
  const layers = depths.map((d) => ({ id: d, name: `Depth ${d}` }));
  return {
    nodes: graph.nodes.map(
      (node) =>
        ({
          id: node.id,
          type: "text",
          x: node.position.x,
          y: node.position.y,
          width: node.width,
          height: node.height,
          text: node.data.content,
          label: node.data.text,
          "x-layer": node.data.depth,
          "x-map": node.data,
        }) as CanvasNode,
    ),
    edges: graph.edges,
    "x-layers": layers,
  };
}

/** The `x-map` payload of a canvas node the view renders for the map. */
export function mapNodeData(node: CanvasNode): MapNodeData | undefined {
  return node["x-map"] as MapNodeData | undefined;
}
