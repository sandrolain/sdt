import { describe, expect, it } from "vitest";
import { bounds, clamp, escapeSvg, fitDistance, smooth } from "./geometry";

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
