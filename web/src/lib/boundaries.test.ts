import { describe, expect, it } from "vitest";
import {
  annotateBoundaries,
  boundaryRects,
  boundaryText,
  nodeText,
  parseBoundaries,
} from "./boundaries";
import type { MindNode } from "./mindmap";

function node(
  content: string,
  children: MindNode[] = [],
  rect?: { x: number; y: number; width: number; height: number },
): MindNode {
  return { content, children, state: rect ? { rect } : undefined };
}

describe("boundaryText / nodeText", () => {
  it("strips list markers, heading prefixes and boundary markers", () => {
    expect(boundaryText("- topic one [B1]")).toBe("topic one");
    expect(boundaryText("## Heading [B]")).toBe("Heading");
    expect(nodeText(node("<strong>Alpha</strong> &amp; Beta"))).toBe("Alpha & Beta");
  });
});

describe("parseBoundaries", () => {
  it("collects membership and titles", () => {
    const md = [
      "- alpha [B1]",
      "- beta",
      "  [B1]: Group title",
      "- gamma [B2]",
      "  [B2]: Other",
    ].join("\n");
    const parsed = parseBoundaries(md);
    expect(parsed.mark.get("alpha")).toBe("1");
    expect(parsed.mark.get("gamma")).toBe("2");
    expect(parsed.mark.has("beta")).toBe(false);
    expect(parsed.titles.get("1")).toBe("Group title");
    expect(parsed.titles.get("2")).toBe("Other");
  });

  it("treats an unnumbered marker as boundary 0", () => {
    const parsed = parseBoundaries("- alpha [B]\n  [B]: Plain");
    expect(parsed.mark.get("alpha")).toBe("0");
    expect(parsed.titles.get("0")).toBe("Plain");
  });
});

describe("annotateBoundaries", () => {
  it("groups only consecutive same-boundary siblings", () => {
    const root = node("root", [node("alpha"), node("beta"), node("gamma"), node("delta")]);
    const parsed = {
      mark: new Map([
        ["alpha", "1"],
        ["beta", "1"],
        ["delta", "1"],
      ]),
      titles: new Map([["1", "G"]]),
    };
    const groups = annotateBoundaries(root, parsed);
    expect(groups).toHaveLength(2);
    expect(groups[0].nodes.map((n) => nodeText(n))).toEqual(["alpha", "beta"]);
    expect(groups[0].title).toBe("G");
    expect(groups[1].nodes.map((n) => nodeText(n))).toEqual(["delta"]);
  });

  it("scans nested levels", () => {
    const root = node("root", [node("parent", [node("child1"), node("child2")])]);
    const parsed = {
      mark: new Map([
        ["child1", "9"],
        ["child2", "9"],
      ]),
      titles: new Map<string, string>(),
    };
    const groups = annotateBoundaries(root, parsed);
    expect(groups).toHaveLength(1);
    expect(groups[0].nodes).toHaveLength(2);
  });
});

describe("boundaryRects", () => {
  it("unions member rects with padding and skips members without rects", () => {
    const a = node("a", [], { x: 10, y: 10, width: 50, height: 20 });
    const b = node("b", [], { x: 10, y: 40, width: 80, height: 20 });
    const rects = boundaryRects(
      [
        { id: "1", title: "G", nodes: [a, b] },
        { id: "2", nodes: [node("c")] },
      ],
      5,
    );
    expect(rects).toHaveLength(1);
    expect(rects[0]).toEqual({ id: "1", title: "G", x: 5, y: 5, width: 90, height: 60 });
  });
});
