import { describe, expect, it } from "vitest";
import type { RelResponse } from "./api";
import { groupRelations, relationCounts } from "./relations";

const RESP: RelResponse = {
  id: "a",
  title: "Alpha",
  outbound: {
    depends_on: [{ target: "b", title: "Beta", kind: "relation", path: "context/wiki/b.md" }],
    refers_to: [
      { target: "c", title: "Gamma", kind: "link", label: "see", path: "context/wiki/c.md" },
    ],
  },
  inbound: {
    part_of: [{ source: "d", title: "Delta", kind: "relation", path: "context/wiki/d.md" }],
    depends_on: [{ source: "e", title: "Epsilon", kind: "relation", path: "context/wiki/e.md" }],
  },
};

describe("groupRelations", () => {
  it("groups by verb with direction and stable ordering", () => {
    const groups = groupRelations(RESP);
    expect(groups.map((g) => g.verb)).toEqual(["depends_on", "part_of", "refers_to"]);
    const depends = groups.find((g) => g.verb === "depends_on")!;
    expect(depends.outbound.map((i) => i.id)).toEqual(["b"]);
    expect(depends.inbound.map((i) => i.id)).toEqual(["e"]);
    expect(depends.inbound[0].direction).toBe("inbound");
  });

  it("falls back to the neighbor id for the title and keeps labels", () => {
    const groups = groupRelations({
      id: "a",
      title: "A",
      outbound: { refers_to: [{ target: "zzz", kind: "link", path: "p" }] },
    });
    expect(groups[0].outbound[0].title).toBe("zzz");
  });

  it("returns no groups without relations", () => {
    expect(groupRelations({ id: "a", title: "A" })).toEqual([]);
  });
});

describe("relationCounts", () => {
  it("counts inbound and outbound rows", () => {
    expect(relationCounts(RESP)).toEqual({ inbound: 2, outbound: 2 });
    expect(relationCounts({ id: "a", title: "A" })).toEqual({ inbound: 0, outbound: 0 });
  });
});
