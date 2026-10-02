import { describe, expect, it } from "vitest";
import { annotateBoundaries, annotateSummaries, boundaryRects, nodeText } from "./boundaries";
import { measureTree } from "./mapMetrics";
import { parseMapDocument } from "./mindmap";

/** Parse a map fragment and measure it, so ids, boxes and markers are real. */
function parse(md: string) {
  return parseMapDocument(`# Root\n\n${md}`);
}

function measured(md: string) {
  return measureTree(parse(md));
}

function labels(nodes: { content: string }[]): string[] {
  return nodes.map((n) => nodeText(n));
}

describe("annotateBoundaries", () => {
  it("groups only consecutive same-boundary siblings and titles them", () => {
    const root = parse(
      ["## a [B1]", "## b [B1]", "## c", "## d [B1]", "[B1]: Wrap title"].join("\n\n"),
    );
    const tree = measureTree(root);
    const groups = annotateBoundaries(tree, root.payload?.titles);
    expect(groups).toHaveLength(2);
    expect(labels(groups[0].nodes)).toEqual(["a", "b"]);
    expect(groups[0].title).toBe("Wrap title");
    expect(labels(groups[1].nodes)).toEqual(["d"]);
  });

  it("scans nested levels and reads the marker off the node's own annotations", () => {
    const root = parse("## parent\n\n- child1 [B2]\n- child2 [B2]\n");
    const tree = measureTree(root);
    const groups = annotateBoundaries(tree, root.payload?.titles);
    expect(groups).toHaveLength(1);
    expect(labels(groups[0].nodes)).toEqual(["child1", "child2"]);
    expect(groups[0].nodes.map((n) => n.id)).toEqual(["n0.1.1", "n0.1.2"]);
    // The label itself carries no marker: it was stripped at parse time.
    expect(nodeText(groups[0].nodes[0])).toBe("child1");
  });

  it("does not share a membership between two nodes with the same text", () => {
    const tree = measureTree(parseMapDocument("# Root\n\n- dup [B1]\n- other\n- dup [B1]\n"));
    const groups = annotateBoundaries(tree);
    expect(groups).toHaveLength(2);
    expect(labels(groups[0].nodes)).toEqual(["dup"]);
    expect(labels(groups[1].nodes)).toEqual(["dup"]);
  });

  it("treats an unnumbered marker as boundary 0", () => {
    const root = parse("## a [B]\n## b [B]\n[B]: Plain\n");
    const groups = annotateBoundaries(measureTree(root), root.payload?.titles);
    expect(groups[0].id).toBe("0");
    expect(groups[0].title).toBe("Plain");
  });
});

describe("annotateSummaries", () => {
  it("groups consecutive same-summary siblings under their own title map", () => {
    const root = parse(
      ["## a [S1]", "## b [S1]", "## c [B1]", "[B1]: B title", "[S1]: S title"].join("\n\n"),
    );
    const summaries = annotateSummaries(measureTree(root), root.payload?.titles);
    expect(summaries).toHaveLength(1);
    expect(labels(summaries[0].nodes)).toEqual(["a", "b"]);
    expect(summaries[0].title).toBe("S title");
  });

  it("keeps a boundary and a summary on the same node independent", () => {
    const root = parse("## a [B1][S1]\n## b [B1][S1]\n");
    const tree = measureTree(root);
    expect(annotateBoundaries(tree, root.payload?.titles)).toHaveLength(1);
    expect(annotateSummaries(tree, root.payload?.titles)).toHaveLength(1);
  });
});

describe("boundaryRects", () => {
  it("unions the member rects with padding and skips members without one", () => {
    const tree = measured("## a [B1]\n\n## b [B1]");
    const rects = new Map([
      ["n0.1", { x: 10, y: 10, width: 50, height: 20 }],
      ["n0.2", { x: 10, y: 40, width: 80, height: 20 }],
    ]);
    const groups = annotateBoundaries(tree);
    const out = boundaryRects(
      [...groups, { id: "9", nodes: [{ id: "nope" } as (typeof groups)[number]["nodes"][number]] }],
      rects,
      5,
    );
    expect(out).toHaveLength(1);
    expect(out[0]).toEqual({ id: "1", title: undefined, x: 5, y: 5, width: 90, height: 60 });
  });

  it("returns nothing when no group has geometry", () => {
    const tree = measured("## a [B1]\n\n## b [B1]");
    expect(boundaryRects(annotateBoundaries(tree), new Map())).toEqual([]);
  });
});
