import type { Markmap } from "markmap-view";
import type { GroupHull } from "./groupHull";

/** Draw convex-hull group boundaries behind the nodes (layout coords). */
export function drawGroupHullLayer(mm: Markmap, hulls: GroupHull[]): void {
  const layer = mm.g.selectAll<SVGGElement, unknown>("g.mm-groups").data([null]);
  const merged = layer.join("g").attr("class", "mm-groups");
  merged.lower();
  merged
    .selectAll<SVGPathElement, GroupHull>("path.mm-group-hull")
    .data(hulls, (d) => d.id)
    .join("path")
    .attr("class", "mm-group-hull")
    .attr("d", (d: GroupHull) => d.path)
    .attr("fill", (d: GroupHull) => d.color)
    .attr("fill-opacity", 0.08)
    .attr("stroke", (d: GroupHull) => d.color)
    .attr("stroke-dasharray", "6 4");
  merged
    .selectAll<SVGTextElement, GroupHull>("text.mm-group-label")
    .data(hulls, (d) => d.id)
    .join("text")
    .attr("class", "mm-group-label")
    .attr("x", (d: GroupHull) => d.labelX)
    .attr("y", (d: GroupHull) => d.labelY)
    .attr("fill", (d: GroupHull) => d.color)
    .attr("font-size", 12)
    .text((d: GroupHull) => d.id);
}
