import { describe, expect, it } from "vitest";
import { annotateBoundaries, annotateSummaries, boundaryRects } from "./boundaries";
import { annotateGroups } from "./groups";
import { groupHulls } from "./groupHull";
import { layoutMap } from "./mapLayout";
import { escapeXml, mapToSvg, type MapExportInput } from "./mapExport";
import { arrow, edgeGeom } from "./jsoncanvas/geometry";
import type { CanvasNode } from "./jsoncanvas/document";
import { buildMapGraph, nodeRects } from "./mapModel";
import { measureTree } from "./mapMetrics";
import { parseMapDocument } from "./mindmap";

/** Build the exact model the view hands to the exporter. */
function model(md: string, collapsed: ReadonlySet<string> = new Set()): MapExportInput {
  const root = parseMapDocument(md);
  const tree = measureTree(root);
  const { positions, bounds } = layoutMap(tree, "balanced");
  const { nodes, edges } = buildMapGraph(tree, positions, { collapsed });
  const rects = nodeRects(nodes);
  const prune = (node: ReturnType<typeof measureTree>): ReturnType<typeof measureTree> => ({
    ...node,
    children: node.children.filter((c) => nodes.some((n) => n.id === c.id)).map(prune),
  });
  const visible = prune(tree);
  return {
    nodes,
    edges,
    bounds,
    boundaries: boundaryRects(annotateBoundaries(visible, root.payload?.titles), rects, 10),
    summaries: boundaryRects(annotateSummaries(visible, root.payload?.titles), rects, 10),
    hulls: groupHulls(annotateGroups(visible), rects),
  };
}

const MD = [
  "# Root",
  "",
  "- alpha [B1] [1]",
  "- beta [B1] [^1](Cool) [N:remember] [!star]",
  "",
  "[B1]: Wrap",
  "",
  "- gamma #group/hull",
].join("\n");

describe("escapeXml", () => {
  it("escapes every character that would break the document", () => {
    expect(escapeXml(`a & b < c > d " e ' f`)).toBe("a &amp; b &lt; c &gt; d &quot; e &apos; f");
  });
});

describe("mapToSvg", () => {
  it("is deterministic for the same model", () => {
    const input = model(MD);
    expect(mapToSvg(input)).toBe(mapToSvg(input));
  });

  it("emits a standalone document with a sized viewBox around the bounds", () => {
    const { bounds } = model(MD);
    const svg = mapToSvg({ ...model(MD), title: "Topic map" });
    expect(svg.startsWith('<svg xmlns="http://www.w3.org/2000/svg"')).toBe(true);
    expect(svg).toContain(`viewBox="${bounds.minX - 10} ${bounds.minY - 10}`);
    expect(svg).toContain("<title>Topic map</title>");
    expect(svg.endsWith("</svg>")).toBe(true);
  });

  it("draws every visible node, its text and its sticker names", () => {
    const input = model(MD);
    const svg = mapToSvg(input);
    for (const node of input.nodes) {
      expect(svg).toContain(`<rect x="${node.position.x}"`);
    }
    expect(svg).toContain(">alpha<");
    expect(svg).toContain(">gamma<");
    // A static SVG has no icon font, so a sticker exports as its marker name.
    expect(svg).toContain(">star<");
  });

  it("draws the wrap shapes and the group hull", () => {
    const input = model(MD);
    const svg = mapToSvg(input);
    expect(svg).toContain(">Wrap<");
    expect(svg).toContain(">hull<");
    expect(svg.match(/stroke-dasharray="6 4"/g)?.length).toBeGreaterThan(1);
  });

  it("draws a relationship edge with its title, dashed", () => {
    const svg = mapToSvg(model(MD));
    expect(svg).toContain(">Cool<");
    expect(svg).toContain('stroke-dasharray="5 4"');
  });

  it("escapes label text instead of injecting it", () => {
    const svg = mapToSvg(model("# Root\n\n- <script>alert(1)</script> [B]\n"));
    expect(svg).not.toContain("<script>");
    expect(svg).toContain("&lt;script&gt;");
  });

  it("exports the current collapsed state, not the whole tree", () => {
    const full = model("# Root\n\n- parent\n  - leaf\n");
    const folded = model("# Root\n\n- parent\n  - leaf\n", new Set(["n0.1"]));
    expect(full.nodes).toHaveLength(3);
    expect(folded.nodes).toHaveLength(2);
    expect(mapToSvg(folded)).not.toContain(">leaf<");
  });

  it("handles a single-node map without a degenerate viewBox", () => {
    const svg = mapToSvg(model("# Root\n"));
    expect(svg).toMatch(/width="\d+"/);
    expect(svg).not.toContain("NaN");
  });

  it("draws each edge with the shared side-anchored bezier, an arrow head and the mid label", () => {
    const input = model(MD);
    const svg = mapToSvg(input);
    const byId: Record<string, CanvasNode> = Object.fromEntries(
      input.nodes.map((n) => [
        n.id,
        {
          id: n.id,
          type: "text",
          x: n.position.x,
          y: n.position.y,
          width: n.width,
          height: n.height,
        },
      ]),
    );
    for (const edge of input.edges) {
      const g = edgeGeom(edge, byId);
      if (!g) continue;
      expect(svg).toContain(`d="${g.d}"`);
      expect(svg).toContain(`points="${arrow(g.q, g.db)}"`);
    }
  });
});
