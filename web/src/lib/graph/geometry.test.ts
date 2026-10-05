import { describe, expect, it } from "vitest";
import {
  bounds,
  clamp,
  escapeSvg,
  fitDistance,
  perpendicular3,
  reciprocalSign,
  smooth,
} from "./geometry";

describe("clamp / smooth", () => {
  it("clamps to the bounds", () => {
    expect(clamp(5, 0, 10)).toBe(5);
    expect(clamp(-1, 0, 10)).toBe(0);
    expect(clamp(11, 0, 10)).toBe(10);
  });

  it("smooths between the edges to 0 and 1", () => {
    expect(smooth(0, 10, 0)).toBe(0);
    expect(smooth(0, 10, 10)).toBe(1);
    expect(smooth(0, 10, 5)).toBeCloseTo(0.5, 5);
  });
});

describe("escapeSvg", () => {
  it("escapes the five XML specials", () => {
    expect(escapeSvg(`a&b<c>"d'e`)).toBe("a&amp;b&lt;c&gt;&quot;d&apos;e");
  });
});

describe("bounds", () => {
  it("returns the centroid and a positive radius", () => {
    const pos = new Float32Array([0, 0, 0, 10, 0, 0]);
    const b = bounds(pos, 2);
    expect(b.cx).toBeCloseTo(5, 5);
    expect(b.cy).toBeCloseTo(0, 5);
    expect(b.cz).toBeCloseTo(0, 5);
    expect(b.r).toBeGreaterThan(0);
  });

  it("returns the documented empty bounds", () => {
    expect(bounds(new Float32Array(), 0)).toEqual({ cx: 0, cy: 0, cz: 0, r: 40 });
  });
});

describe("fitDistance", () => {
  it("is smaller in 2D than in 3D for the same radius", () => {
    const d3 = fitDistance(100, 50, 1, 0);
    const d2 = fitDistance(100, 50, 1, 1);
    expect(d3).toBeGreaterThan(d2);
    expect(d2).toBeGreaterThanOrEqual(60);
  });

  it("grows with the radius", () => {
    expect(fitDistance(200, 50, 1, 1)).toBeGreaterThan(fitDistance(100, 50, 1, 1));
  });
});

describe("perpendicular3", () => {
  const dot = (p: [number, number, number], d: [number, number, number]) =>
    p[0] * d[0] + p[1] * d[1] + p[2] * d[2];

  it("returns a unit vector orthogonal to the direction", () => {
    for (const d of [
      [1, 0, 0],
      [0, 1, 0],
      [0, 0, 1],
      [1, 2, 3],
      [-4, 0.5, 2],
    ] as [number, number, number][]) {
      const len = Math.hypot(...d);
      const u: [number, number, number] = [d[0] / len, d[1] / len, d[2] / len];
      const p = perpendicular3(u[0], u[1], u[2]);
      expect(Math.hypot(...p)).toBeCloseTo(1, 6);
      expect(dot(p, u)).toBeCloseTo(0, 6);
    }
  });

  it("is orthogonal even when dz is non-zero (the old (-dy, dx, 0) was not)", () => {
    const p = perpendicular3(0, 0, 1);
    expect(p[2]).toBe(0);
    expect(Math.abs(p[0]) + Math.abs(p[1])).toBeCloseTo(1, 6);
  });

  it("returns the zero vector for a degenerate direction", () => {
    expect(perpendicular3(0, 0, 0)).toEqual([0, 0, 0]);
  });
});

describe("reciprocalSign", () => {
  const pair = [
    { a: 0, b: 1 },
    { a: 1, b: 0 },
  ];

  it("gives opposite signs to the two directions of a mutual pair", () => {
    expect(reciprocalSign(0, 1, pair)).toBe(1);
    expect(reciprocalSign(1, 0, pair)).toBe(-1);
  });

  it("gives zero without a reverse edge or for a self-loop", () => {
    expect(reciprocalSign(0, 1, [{ a: 0, b: 1 }])).toBe(0);
    expect(reciprocalSign(0, 0, pair)).toBe(0);
  });
});
