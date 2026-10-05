import { describe, expect, it } from "vitest";
import { adaptToEngine } from "./adapter";
import type { GraphData } from "../graphModel";

const data: GraphData = {
  nodes: [
    {
      id: "a",
      title: "Alpha",
      type: "analysis",
      status: "active",
      tags: ["ui/x"],
      path: "context/a.md",
      summary: "first",
    },
    { id: "b", title: "Beta", type: "plan", status: "active", path: "context/b.md" },
  ],
  edges: [
    { source: "a", target: "b", verb: "depends_on", kind: "relation", label: "dep" },
    { source: "b", target: "a", verb: "refers_to", kind: "link" },
  ],
};

describe("adaptToEngine", () => {
  it("maps SDT fields onto the engine contract", () => {
    const g = adaptToEngine(data, { clusterKey: "type" });
    expect(g.nodes[0]).toMatchObject({
      id: "a",
      label: "Alpha",
      group: "analysis",
      description: "first",
    });
    expect(g.nodes[0].color).toBeDefined();
    expect(g.links[0]).toMatchObject({
      source: "a",
      target: "b",
      type: "depends_on",
      kind: "relation",
      label: "dep",
    });
  });

  it("pre-filters edges by verb and kind", () => {
    const verbs = adaptToEngine(data, { clusterKey: "type", visibleVerbs: new Set(["refers_to"]) });
    expect(verbs.links).toHaveLength(1);
    expect(verbs.links[0].type).toBe("refers_to");
    const kinds = adaptToEngine(data, { clusterKey: "type", visibleKinds: new Set(["relation"]) });
    expect(kinds.links).toHaveLength(1);
    expect(kinds.links[0].kind).toBe("relation");
  });

  it("supports the three cluster keys", () => {
    expect(adaptToEngine(data, { clusterKey: "status" }).nodes[0].group).toBe("active");
    expect(adaptToEngine(data, { clusterKey: "tag-root" }).nodes[0].group).toBe("ui");
    expect(adaptToEngine(data, { clusterKey: "tag-root" }).nodes[1].group).toBe("untagged");
  });

  it("lists all verbs and kinds before filtering", () => {
    const g = adaptToEngine(data, { clusterKey: "type", visibleVerbs: new Set(["refers_to"]) });
    expect(g.allVerbs).toEqual(["depends_on", "refers_to"]);
    expect(g.allKinds).toEqual(["link", "relation"]);
  });

  it("handles an empty graph", () => {
    const g = adaptToEngine({ nodes: [], edges: [] }, { clusterKey: "type" });
    expect(g.nodes).toEqual([]);
    expect(g.links).toEqual([]);
    expect(g.allVerbs).toEqual([]);
  });
});
