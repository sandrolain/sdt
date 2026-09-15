import type { Markmap } from "markmap-view";
import type { BoundaryRect } from "./boundaries";

/** Draw rounded-rect boundary boxes behind the grouped nodes (layout coords). */
export function drawBoundaryLayer(mm: Markmap, rects: BoundaryRect[]): void {
  const layer = mm.g.selectAll<SVGGElement, unknown>("g.mm-boundaries").data([null]);
  const merged = layer.join("g").attr("class", "mm-boundaries");
  merged.lower();
  merged
    .selectAll<SVGRectElement, BoundaryRect>("rect")
    .data(rects)
    .join("rect")
    .attr("x", (d: BoundaryRect) => d.x)
    .attr("y", (d: BoundaryRect) => d.y)
    .attr("width", (d: BoundaryRect) => d.width)
    .attr("height", (d: BoundaryRect) => d.height)
    .attr("rx", 10)
    .attr("ry", 10)
    .attr("fill", "none")
    .attr("stroke", "#fab387")
    .attr("stroke-dasharray", "6 4");
  merged
    .selectAll<SVGTextElement, BoundaryRect>("text")
    .data(rects.filter((r) => Boolean(r.title)))
    .join("text")
    .attr("x", (d: BoundaryRect) => d.x + 8)
    .attr("y", (d: BoundaryRect) => d.y + 15)
    .attr("fill", "#fab387")
    .attr("font-size", 12)
    .text((d: BoundaryRect) => d.title ?? "");
}
