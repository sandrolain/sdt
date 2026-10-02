import { describe, expect, it } from "vitest";
import {
  MAP_LAYOUTS,
  layoutBalanced,
  layoutBounds,
  layoutMap,
  layoutRadial,
  type MapPoint,
} from "./mapLayout";
import { flattenMeasured, measureTree, type MeasuredNode } from "./mapMetrics";
import { parseMapDocument } from "./mindmap";

/** A minimal hand-built topic, so a layout can be tested without a parser. */
type MindNodeish = {
  content: string;
  children: MindNodeish[];
  payload?: { kind: "topic" | "quote" | "code" | "table" | "text" };
};

function node(content: string, children: MindNodeish[] = []): MindNodeish {
  return { content, children, payload: { kind: "topic" } };
}

/** `branches` children of the root, each with `leaves` leaf children. */
function wide(branches: number, leaves: number): MeasuredNode {
  return measureTree(
    node(
      "Root",
      Array.from({ length: branches }, (_, b) =>
        node(
          `branch ${b}`,
          Array.from({ length: leaves }, (_, l) => node(`leaf ${b}.${l}`)),
        ),
      ),
    ),
  );
}

/** A single chain, to exercise depth without breadth. */
function deep(depth: number): MeasuredNode {
  const build = (level: number): MindNodeish =>
    level >= depth ? node(`level ${level}`) : node(`level ${level}`, [build(level + 1)]);
  return measureTree(build(0));
}

/** The first pair of intersecting boxes, or null (edges may touch). */
function firstOverlap(root: MeasuredNode, positions: Map<string, MapPoint>): string | null {
  const boxes = flattenMeasured(root).map((n) => {
    const p = positions.get(n.id)!;
    return {
      id: n.id,
      x0: p.x - n.box.width / 2,
      y0: p.y - n.box.height / 2,
      x1: p.x + n.box.width / 2,
      y1: p.y + n.box.height / 2,
    };
  });
  for (let i = 0; i < boxes.length; i++) {
    for (let j = i + 1; j < boxes.length; j++) {
      const a = boxes[i];
      const b = boxes[j];
      if (a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1) {
        return `${a.id}/${b.id}`;
      }
    }
  }
  return null;
}

/** Assert that no two boxes intersect (edges may touch, interiors may not). */
function expectNoOverlap(root: MeasuredNode, positions: Map<string, MapPoint>): void {
  const hit = firstOverlap(root, positions);
  if (hit) throw new Error(`boxes overlap: ${hit}`);
}

describe("layoutBalanced", () => {
  it("puts the root at the origin and grows both directions", () => {
    const tree = wide(4, 2);
    const positions = layoutBalanced(tree);
    expect(positions.get("n0")).toEqual({ x: 0, y: 0 });
    const xs = tree.children.map((c) => positions.get(c.id)!.x);
    expect(Math.min(...xs)).toBeLessThan(0);
    expect(Math.max(...xs)).toBeGreaterThan(0);
  });

  it("splits the sides by subtree size, not by alternating", () => {
    const tree = wide(4, 1);
    const positions = layoutBalanced(tree);
    const sides = tree.children.map((c) => (positions.get(c.id)!.x > 0 ? "right" : "left"));
    // Two children per side, and both sides get work: a balanced map uses the
    // width instead of growing right only.
    expect(sides.filter((s) => s === "left")).toHaveLength(2);
    expect(sides.filter((s) => s === "right")).toHaveLength(2);
  });

  it("stacks siblings without overlap and centres a parent on its children", () => {
    const tree = wide(3, 3);
    const positions = layoutBalanced(tree);
    expectNoOverlap(tree, positions);
    for (const child of tree.children) {
      const ys = child.children.map((l) => positions.get(l.id)!.y);
      const parentY = positions.get(child.id)!.y;
      expect(parentY).toBeCloseTo((ys[0] + ys[ys.length - 1]) / 2, 6);
    }
  });

  it("keeps a chain monotonic in x", () => {
    const tree = deep(6);
    const positions = layoutBalanced(tree);
    let previous = 0;
    for (const node of flattenMeasured(tree)) {
      const x = positions.get(node.id)!.x;
      expect(Math.abs(x)).toBeGreaterThanOrEqual(previous);
      previous = Math.abs(x);
    }
  });

  it("is deterministic", () => {
    const tree = wide(5, 2);
    expect([...layoutBalanced(tree)]).toEqual([...layoutBalanced(tree)]);
  });
});

describe("layoutRadial", () => {
  it("puts the root at the origin and its children on the first ring", () => {
    const tree = wide(6, 1);
    const positions = layoutRadial(tree);
    expect(positions.get("n0")).toEqual({ x: 0, y: 0 });
    const radii = tree.children.map((c) =>
      Math.hypot(positions.get(c.id)!.x, positions.get(c.id)!.y),
    );
    expect(new Set(radii.map((r) => r.toFixed(3))).size).toBe(1);
    expect(radii[0]).toBeGreaterThan(0);
  });

  it("places every leaf on its own angle without overlap", () => {
    const tree = wide(8, 2);
    const positions = layoutRadial(tree);
    expectNoOverlap(tree, positions);
    const angles = tree.children
      .flatMap((c) => c.children)
      .map((l) => Math.atan2(positions.get(l.id)!.y, positions.get(l.id)!.x));
    expect(new Set(angles.map((a) => a.toFixed(3))).size).toBe(angles.length);
  });

  it("grows outward with depth", () => {
    const tree = deep(5);
    const positions = layoutRadial(tree);
    let previous = -1;
    for (const n of flattenMeasured(tree)) {
      const r = Math.hypot(positions.get(n.id)!.x, positions.get(n.id)!.y);
      expect(r).toBeGreaterThan(previous);
      previous = r;
    }
    expectNoOverlap(tree, positions);
  });

  it("handles a root with a single child", () => {
    const tree = measureTree(node("Root", [node("only", [node("leaf")])]));
    const positions = layoutRadial(tree);
    expect(positions.size).toBe(3);
    expectNoOverlap(tree, positions);
  });

  it("handles a root with no children", () => {
    const tree = measureTree(node("Root"));
    expect([...layoutRadial(tree)]).toEqual([["n0", { x: 0, y: 0 }]]);
  });

  it("is deterministic", () => {
    const tree = wide(6, 2);
    expect([...layoutRadial(tree)]).toEqual([...layoutRadial(tree)]);
  });
});

describe("no overlap, by construction", () => {
  // The claim in the module doc is only worth something if it is swept: flat,
  // deep and mixed shapes, both layouts, every pair of boxes checked.
  it("holds for synthetic maps from 1 to 12 branches with 1 to 4 leaves", () => {
    const bad: string[] = [];
    for (let branches = 1; branches <= 12; branches++) {
      for (let leaves = 1; leaves <= 4; leaves++) {
        for (const shape of ["flat", "deep", "mixed"] as const) {
          const tree = measureTree(
            node(
              "Root",
              Array.from({ length: branches }, (_, b) =>
                node(
                  `b${b}`,
                  Array.from({ length: leaves }, (_, l) =>
                    shape === "flat"
                      ? node(`l${b}.${l}`)
                      : shape === "deep"
                        ? node(`m${b}.${l}`, [node(`g${b}.${l}`)])
                        : l % 2 === 0
                          ? node(`m${b}.${l}`, [node(`g${b}.${l}`)])
                          : node(`l${b}.${l}`),
                  ),
                ),
              ),
            ),
          );
          for (const [kind, layout] of [
            ["balanced", layoutBalanced],
            ["radial", layoutRadial],
          ] as const) {
            const hit = firstOverlap(tree, layout(tree));
            if (hit) bad.push(`${kind} ${branches}x${leaves} ${shape}: ${hit}`);
          }
        }
      }
    }
    expect(bad).toEqual([]);
  });
});

describe("layoutMap", () => {
  it("exposes both layouts and a bounds box that contains every node", () => {
    expect(MAP_LAYOUTS.map((l) => l.id)).toEqual(["balanced", "radial"]);
    for (const kind of MAP_LAYOUTS.map((l) => l.id)) {
      const tree = wide(4, 2);
      const { positions, bounds } = layoutMap(tree, kind);
      for (const n of flattenMeasured(tree)) {
        const p = positions.get(n.id)!;
        expect(p.x - n.box.width / 2).toBeGreaterThanOrEqual(bounds.minX - 1e-9);
        expect(p.x + n.box.width / 2).toBeLessThanOrEqual(bounds.maxX + 1e-9);
        expect(p.y - n.box.height / 2).toBeGreaterThanOrEqual(bounds.minY - 1e-9);
        expect(p.y + n.box.height / 2).toBeLessThanOrEqual(bounds.maxY + 1e-9);
      }
      expect(bounds.width).toBeGreaterThan(0);
      expect(bounds.height).toBeGreaterThan(0);
    }
  });

  it("reports the root's own box as the bounds of a single-node map", () => {
    const tree = measureTree(node("Root"));
    expect(layoutBounds(tree, layoutMap(tree, "balanced").positions)).toEqual({
      minX: -120,
      minY: -18,
      maxX: 120,
      maxY: 18,
      width: 240,
      height: 36,
    });
  });

  it("parses a real map document and lays it out", () => {
    const tree = measureTree(parseMapDocument("# Root\n\n- a\n- b\n  - c\n\n## Branch\n\n- d\n"));
    for (const kind of ["balanced", "radial"] as const) {
      expectNoOverlap(tree, layoutMap(tree, kind).positions);
    }
  });
});
