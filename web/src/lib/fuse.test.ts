import { describe, expect, it } from "vitest";
import { extractMapRefs, fuseMap, type MapRef } from "./fuse";
import type { MindNode } from "./mindmap";

function node(content: string, children: MindNode[] = []): MindNode {
  return { content, children };
}

function mapRef(id: string, title: string, root: MindNode): MapRef {
  return { id, title, root };
}

describe("extractMapRefs", () => {
  it("finds referenced map ids from wiki and docs hrefs", () => {
    const root = node("root", [
      node('<a href="/wiki/topic.map">Topic</a>'),
      node('<a href="/docs/context/notes/plan.map.md">Plan</a>'),
      node('<a href="https://example.com/x.md">Ext</a>'),
    ]);
    const index = new Map<string, MapRef>([
      ["topic.map", mapRef("topic.map", "Topic", node("t"))],
      ["context/notes/plan.map.md", mapRef("context/notes/plan.map.md", "Plan", node("p"))],
    ]);
    expect(extractMapRefs(root, index).sort()).toEqual(["context/notes/plan.map.md", "topic.map"]);
  });
});

describe("fuseMap", () => {
  const refs = new Map<string, MapRef>([
    ["b", mapRef("b", "Map B", node("B", [node("B1"), node('<a href="/wiki/c">C</a>')]))],
    ["c", mapRef("c", "Map C", node("C", [node("C1")]))],
    ["a", mapRef("a", "Map A", node("A", [node("A1")]))],
  ]);

  it("imports referenced maps under labeled nodes", () => {
    const root = node("base", [node('<a href="/wiki/b">B</a>')]);
    const { root: fused, stats } = fuseMap("base", root, refs, { maxDepth: 2, maxNodes: 100 });
    expect(stats.imported).toEqual(["b", "c"]);
    const all = JSON.stringify(fused);
    expect(all).toContain("[Map B]");
    expect(all).toContain("[Map C]");
  });

  it("dedupes an already-imported map", () => {
    const root = node("base", [
      node('<a href="/wiki/b">B</a>'),
      node('<a href="/wiki/b">B again</a>'),
    ]);
    const { stats } = fuseMap("base", root, refs, { maxDepth: 2, maxNodes: 100 });
    expect(stats.imported).toEqual(["b", "c"]);
    expect(stats.deduped).toContain("b");
  });

  it("detects cycles against the base map", () => {
    const root = node("base", [node('<a href="/wiki/a">A</a>')]);
    const { stats } = fuseMap("a", root, refs, { maxDepth: 2, maxNodes: 100 });
    expect(stats.cycles).toEqual(["a"]);
    expect(stats.imported).toEqual([]);
  });

  it("respects the depth budget", () => {
    const root = node("base", [node('<a href="/wiki/b">B</a>')]);
    const { stats } = fuseMap("base", root, refs, { maxDepth: 1, maxNodes: 100 });
    expect(stats.imported).toEqual(["b"]);
  });

  it("respects the node budget", () => {
    const root = node("base", [node('<a href="/wiki/b">B</a>')]);
    const { stats } = fuseMap("base", root, refs, { maxDepth: 3, maxNodes: 2 });
    expect(stats.truncated).toBe(true);
  });
});
