import { describe, expect, it } from "vitest";
import { clusterOf, clusterPalette, type GraphData } from "./graphModel";

const DATA: GraphData = {
  nodes: [
    {
      id: "a",
      title: "Alpha",
      type: "concept",
      status: "active",
      tags: ["backend/auth"],
      path: "context/wiki/a.md",
    },
    {
      id: "b",
      title: "Beta",
      type: "module",
      status: "draft",
      tags: ["ui"],
      path: "context/wiki/b.md",
    },
    { id: "c", title: "Gamma", type: "concept", status: "active", path: "context/wiki/c.md" },
  ],
  edges: [
    { source: "a", target: "b", verb: "depends_on", kind: "relation" },
    { source: "a", target: "c", verb: "refers_to", kind: "link" },
  ],
};

describe("clusterOf", () => {
  it("resolves type, tag-root and status", () => {
    expect(clusterOf(DATA.nodes[0], "type")).toBe("concept");
    expect(clusterOf(DATA.nodes[0], "tag-root")).toBe("backend");
    expect(clusterOf(DATA.nodes[1], "tag-root")).toBe("ui");
    expect(clusterOf(DATA.nodes[0], "status")).toBe("active");
    expect(clusterOf(DATA.nodes[2], "status")).toBe("active");
  });

  it("falls back to unknown when a dimension is missing", () => {
    expect(clusterOf(DATA.nodes[2], "type")).toBe("concept");
    expect(clusterOf({ id: "x", title: "X", path: "p" }, "type")).toBe("unknown");
    expect(clusterOf({ id: "x", title: "X", path: "p" }, "tag-root")).toBe("untagged");
  });
});

describe("clusterPalette", () => {
  it("assigns stable colors by sorted cluster id", () => {
    const palette = clusterPalette(DATA.nodes, "type");
    expect(palette.get("concept")).toBeDefined();
    expect(palette.get("module")).toBeDefined();
    expect(palette.get("concept")).not.toBe(palette.get("module"));
    const again = clusterPalette(DATA.nodes, "type");
    expect(again.get("concept")).toBe(palette.get("concept"));
  });
});
