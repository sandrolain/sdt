import { describe, expect, it } from "vitest";
import { METRICS, labelLines, measureNode, measureTree, wrapLabel } from "./mapMetrics";
import { parseMapDocument, type MindNode } from "./mindmap";

describe("labelLines", () => {
  it("strips tags, decodes entities and keeps <br> as a line break", () => {
    expect(labelLines("<strong>Alpha</strong> &amp; Beta")).toEqual(["Alpha & Beta"]);
    expect(labelLines("one<br>two")).toEqual(["one", "two"]);
    expect(labelLines("")).toEqual([""]);
  });
});

describe("wrapLabel", () => {
  it("keeps a short line and breaks a long one on word boundaries", () => {
    expect(wrapLabel("short", 20)).toEqual(["short"]);
    expect(wrapLabel("alpha beta gamma delta", 11)).toEqual(["alpha beta", "gamma delta"]);
  });

  it("puts an over-long word on its own line rather than dropping it", () => {
    expect(wrapLabel("supercalifragilistic", 5)).toEqual(["supercalifragilistic"]);
  });
});

describe("measureNode", () => {
  it("gives the root a wider box and grows the height with the wrapped lines", () => {
    const leaf = measureNode("topic", "topic", 1);
    const root = measureNode("topic", "topic", 0);
    expect(root.width).toBeGreaterThan(leaf.width);
    expect(leaf.height).toBe(METRICS.lineHeight + 2 * METRICS.paddingY);
    const long = measureNode("word ".repeat(120).trim(), "topic", 1);
    expect(long.lines.length).toBeGreaterThan(1);
    expect(long.height).toBe(long.lines.length * METRICS.lineHeight + 2 * METRICS.paddingY);
  });

  it("gives a code node its own width", () => {
    expect(measureNode("x", "code", 1).width).toBe(METRICS.widthCode);
  });
});

describe("measureTree", () => {
  const md = ["# Root", "", "- alpha [B1]", "  - nested **bold**", "- beta", "", "> quoted"].join(
    "\n",
  );
  const tree = measureTree(parseMapDocument(md));

  it("assigns stable DFS-path ids with the root at n0", () => {
    expect(tree.id).toBe("n0");
    expect(tree.children.map((c) => c.id)).toEqual(["n0.1", "n0.2", "n0.3"]);
    expect(tree.children[0].children.map((c) => c.id)).toEqual(["n0.1.1"]);
    expect(tree.children[0].parent).toBe("n0");
    expect(tree.depth).toBe(0);
  });

  it("carries the node kind, the markers and the label lines", () => {
    expect(tree.children[0].markers.boundary).toBe("1");
    expect(tree.children[2].kind).toBe("quote");
    expect(tree.children[0].children[0].lines).toEqual(["nested bold"]);
    expect(tree.box.width).toBe(METRICS.widthRoot);
  });

  it("collects the hrefs of the label for click-to-open", () => {
    const withLink = measureTree(parseMapDocument("- see [[accounts|Accounts]]\n"));
    expect(withLink.children[0].links).toEqual(["/wiki/accounts"]);
  });

  it("is deterministic: the same document measures identically twice", () => {
    const again = measureTree(parseMapDocument(md));
    expect(JSON.stringify(again)).toBe(JSON.stringify(tree));
  });

  it("measures a tree built by hand the same way as a parsed one", () => {
    const hand: MindNode = {
      content: "Root",
      children: [{ content: "child", children: [], payload: { kind: "topic" } }],
      payload: { kind: "topic" },
    };
    expect(measureTree(hand).children[0].id).toBe("n0.1");
  });
});
