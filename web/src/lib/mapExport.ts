import type { BoundaryRect } from "./boundaries";
import type { GroupHull } from "./groupHull";
import type { MapFlowEdge, MapFlowNode } from "./mapModel";
import { METRICS } from "./mapMetrics";
import type { MapBounds, MapPoint } from "./mapLayout";

/**
 * Export by serializing the model, not the DOM (plan D12): the same boxes, the
 * same shapes and the same text the viewer draws, emitted as a standalone SVG.
 * React Flow ships no exporter and `html-to-image` is not a dependency, so the
 * serializer is ours and deterministic — the output is testable without a
 * browser, and PNG is that SVG rasterized through `Image` + `canvas`.
 */

/** The palette, shared with the overlay so an export matches what is on screen. */
const BOUNDARY_COLOR = "#fab387";
const SUMMARY_COLOR = "#94e2d5";
const EDGE_COLOR = "#6c7086";
const RELATION_COLOR = "#f38ba8";
const NODE_BG = "#1e1e2e";
const NODE_FG = "#cdd6f4";
const NODE_BORDER = "#45475a";
const ROOT_BORDER = "#89b4fa";
const BACKGROUND = "#11111b";

export interface MapExportInput {
  nodes: MapFlowNode[];
  edges: MapFlowEdge[];
  boundaries: BoundaryRect[];
  summaries: BoundaryRect[];
  hulls: GroupHull[];
  bounds: MapBounds;
  title?: string;
}

const XML_ESCAPES: Record<string, string> = {
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  '"': "&quot;",
  "'": "&apos;",
};

/** XML-escape text; the export is a document, not a DOM injection. */
export function escapeXml(text: string): string {
  return text.replace(/[&<>"']/g, (c) => XML_ESCAPES[c]);
}

/** Straight edge between two node centres, clipped at the node box. */
function edgePath(from: MapPoint, to: MapPoint, fromBox: Size, toBox: Size): string {
  const dx = to.x - from.x;
  const dy = to.y - from.y;
  const length = Math.hypot(dx, dy) || 1;
  const ux = dx / length;
  const uy = dy / length;
  // Clip at the box edge: an axis-aligned box reaches half its larger side.
  const fromHalf = Math.max(fromBox.width, fromBox.height) / 2;
  const toHalf = Math.max(toBox.width, toBox.height) / 2;
  return `M${from.x + ux * fromHalf} ${from.y + uy * fromHalf}L${to.x - ux * toHalf} ${to.y - uy * toHalf}`;
}

interface Size {
  width: number;
  height: number;
}

function nodeBox(node: MapFlowNode): Size {
  return { width: node.initialWidth ?? 0, height: node.initialHeight ?? 0 };
}

function nodeCentre(node: MapFlowNode): MapPoint {
  return {
    x: node.position.x + (node.initialWidth ?? 0) / 2,
    y: node.position.y + (node.initialHeight ?? 0) / 2,
  };
}

function shapeSvg(rect: BoundaryRect, color: string, dash: string): string {
  const title = rect.title
    ? `<text x="${rect.x + 8}" y="${rect.y + 15}" fill="${color}" font-size="12" font-family="system-ui, sans-serif">${escapeXml(rect.title)}</text>`
    : "";
  return `<rect x="${rect.x}" y="${rect.y}" width="${rect.width}" height="${rect.height}" rx="10" ry="10" fill="none" stroke="${color}" stroke-dasharray="${dash}" stroke-width="1.5"/>${title}`;
}

/** Serialize the map model to a standalone SVG document. */
export function mapToSvg(input: MapExportInput): string {
  const { nodes, edges, boundaries, summaries, hulls, bounds } = input;
  const padding = METRICS.shapePadding;
  const width = Math.max(bounds.width + padding * 2, 1);
  const height = Math.max(bounds.height + padding * 2, 1);
  const byId = new Map(nodes.map((n) => [n.id, n]));

  const hullSvg = hulls
    .map(
      (hull) =>
        `<path d="${hull.path}" fill="${hull.color}" fill-opacity="0.08" stroke="${hull.color}" stroke-dasharray="6 4" stroke-width="1.5"/>` +
        `<text x="${hull.labelX}" y="${hull.labelY}" fill="${hull.color}" font-size="12" font-family="system-ui, sans-serif">${escapeXml(hull.id)}</text>`,
    )
    .join("");
  const boundarySvg = boundaries.map((rect) => shapeSvg(rect, BOUNDARY_COLOR, "6 4")).join("");
  const summarySvg = summaries.map((rect) => shapeSvg(rect, SUMMARY_COLOR, "none")).join("");

  const edgeSvg = edges
    .map((edge) => {
      const from = byId.get(edge.source);
      const to = byId.get(edge.target);
      if (!from || !to) return "";
      const relation = edge.id.startsWith("r:");
      const d = edgePath(nodeCentre(from), nodeCentre(to), nodeBox(from), nodeBox(to));
      const label = edge.label
        ? `<text x="${(nodeCentre(from).x + nodeCentre(to).x) / 2}" y="${(nodeCentre(from).y + nodeCentre(to).y) / 2 - 6}" fill="${RELATION_COLOR}" font-size="11" text-anchor="middle" font-family="system-ui, sans-serif">${escapeXml(String(edge.label))}</text>`
        : "";
      return (
        `<path d="${d}" fill="none" stroke="${relation ? RELATION_COLOR : EDGE_COLOR}" stroke-width="1.5"` +
        `${relation ? ' stroke-dasharray="5 4"' : ""}/>${label}`
      );
    })
    .join("");

  const nodeSvg = nodes
    .map((node) => {
      const { width: w, height: h } = nodeBox(node);
      const x = node.position.x;
      const y = node.position.y;
      const border =
        node.data.kind === "topic" && node.data.depth === 0 ? ROOT_BORDER : NODE_BORDER;
      const lines = node.data.text ? node.data.text.split("\n") : [""];
      const text = lines
        .map(
          (line, i) =>
            `<tspan x="${x + METRICS.paddingX}" y="${y + METRICS.paddingY + METRICS.nodeFontSize + i * METRICS.lineHeight}">${escapeXml(line)}</tspan>`,
        )
        .join("");
      const stickers = node.data.stickers
        .map(
          (marker) =>
            `<text x="${x + w - 10}" y="${y + METRICS.paddingY + METRICS.nodeFontSize}" fill="${RELATION_COLOR}" font-size="10" text-anchor="end" font-family="system-ui, sans-serif">${escapeXml(marker)}</text>`,
        )
        .join("");
      return (
        `<g><rect x="${x}" y="${y}" width="${w}" height="${h}" rx="8" ry="8" fill="${NODE_BG}" stroke="${border}" stroke-width="1.5"/>` +
        `<text fill="${NODE_FG}" font-size="${METRICS.nodeFontSize}" font-family="system-ui, sans-serif">${text}</text>${stickers}</g>`
      );
    })
    .join("");

  const title = input.title ? `<title>${escapeXml(input.title)}</title>` : "";
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" width="${Math.round(width)}" height="${Math.round(height)}" ` +
    `viewBox="${bounds.minX - padding} ${bounds.minY - padding} ${width} ${height}">${title}` +
    `<rect x="${bounds.minX - padding}" y="${bounds.minY - padding}" width="${width}" height="${height}" fill="${BACKGROUND}"/>` +
    `${hullSvg}${boundarySvg}${summarySvg}${edgeSvg}${nodeSvg}</svg>`
  );
}

/**
 * Rasterize an SVG string to a PNG blob. Browser-only (it needs `Image` and a
 * canvas); the failure path is explicit, never a silent no-op.
 */
export function svgToPng(svg: string, scale = 2): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const blobUrl = URL.createObjectURL(new Blob([svg], { type: "image/svg+xml;charset=utf-8" }));
    const image = new Image();
    image.onload = () => {
      const canvas = document.createElement("canvas");
      canvas.width = Math.max(1, Math.round(image.width * scale));
      canvas.height = Math.max(1, Math.round(image.height * scale));
      const ctx = canvas.getContext("2d");
      if (!ctx) {
        URL.revokeObjectURL(blobUrl);
        reject(new Error("PNG export needs a 2d canvas context"));
        return;
      }
      ctx.scale(scale, scale);
      ctx.drawImage(image, 0, 0);
      URL.revokeObjectURL(blobUrl);
      canvas.toBlob((blob) => {
        if (blob) resolve(blob);
        else reject(new Error("PNG export produced no blob"));
      }, "image/png");
    };
    image.onerror = () => {
      URL.revokeObjectURL(blobUrl);
      reject(new Error("PNG export could not load the serialized SVG"));
    };
    image.src = blobUrl;
  });
}

/** Trigger a browser download for exported bytes. */
export function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}
