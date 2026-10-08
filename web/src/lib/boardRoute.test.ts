import { describe, expect, it } from "vitest";
import { boardPath, parseBoardSplat, buildLevels } from "./boardRoute";
import type { BoardModel } from "./canvas";

function splatOf(path: string): string {
  return path.replace(/^\/wiki\/board\/?/, "");
}

describe("boardRoute", () => {
  it("builds the three route shapes", () => {
    expect(boardPath("", [])).toBe("/wiki/board");
    expect(boardPath("context/wiki/x.canvas", [])).toBe("/wiki/board/context/wiki/x.canvas");
    expect(boardPath("context/wiki/x.canvas", ["a1", "b2"])).toBe(
      "/wiki/board/context/wiki/x.canvas/e/a1/e/b2",
    );
    expect(boardPath("", ["a1"])).toBe("/wiki/board/e/a1");
  });

  it("parses every built route back to its source + ids", () => {
    const cases: [string, string[]][] = [
      ["", []],
      ["context/wiki/x.canvas", []],
      ["context/wiki/x.canvas", ["a1", "b2"]],
      ["", ["a1"]],
    ];
    for (const [file, ids] of cases) {
      expect(parseBoardSplat(splatOf(boardPath(file, ids)))).toEqual({ file, ids });
    }
  });

  it("encodes a node id per segment so a slash survives the route", () => {
    expect(parseBoardSplat(splatOf(boardPath("context/wiki/x.canvas", ["a/b"]))).ids).toEqual([
      "a/b",
    ]);
  });

  it("builds levels from nested nodes and stops at the deepest resolvable one", () => {
    const root = {
      nodes: [
        {
          id: "nc",
          type: "nested-canvas",
          title: "Inv",
          canvas: { nodes: [], edges: [] },
        },
      ],
      edges: [],
    } as unknown as BoardModel;
    expect(buildLevels(root, ["nc"]).map((l) => l.label)).toEqual(["Inv"]);
    expect(buildLevels(root, ["nc", "missing"])).toHaveLength(1);
    expect(buildLevels(root, ["missing"])).toHaveLength(0);
  });
});
