import { describe, expect, it } from "vitest";
import { clampScale, fitTransform, panBy, screenToWorld, worldToScreen, zoomAt } from "./boardView";

const BOUNDS = { minX: 0, minY: 0, maxX: 200, maxY: 100, width: 200, height: 100 };

describe("fitTransform", () => {
  it("fits and centers the board with padding", () => {
    const t = fitTransform(BOUNDS, { width: 400, height: 300 }, 40);
    // 200x100 into 320x220 → scale 1.6 capped to 1
    expect(t.scale).toBe(1);
    expect(t.x).toBe(100); // pad 40 + (320-200)/2
    expect(t.y).toBe(100); // pad 40 + (220-100)/2
  });

  it("shrinks large boards", () => {
    const t = fitTransform(
      { ...BOUNDS, width: 2000, height: 1000, maxX: 2000, maxY: 1000 },
      { width: 400, height: 300 },
      40,
    );
    expect(t.scale).toBeCloseTo(0.16, 5);
  });

  it("never exceeds scale 1 for tiny boards", () => {
    expect(
      fitTransform({ ...BOUNDS, width: 10, height: 10 }, { width: 400, height: 300 }).scale,
    ).toBe(1);
  });
});

describe("zoomAt", () => {
  it("keeps the anchor point stationary", () => {
    const t = { x: 0, y: 0, scale: 1 };
    const next = zoomAt(t, 2, 100, 50);
    expect(next.scale).toBe(2);
    expect(worldToScreen(next, screenToWorld(t, 100, 50).x, screenToWorld(t, 100, 50).y)).toEqual({
      x: 100,
      y: 50,
    });
  });

  it("clamps the scale", () => {
    expect(zoomAt({ x: 0, y: 0, scale: 1 }, 100, 0, 0).scale).toBe(4);
    expect(zoomAt({ x: 0, y: 0, scale: 1 }, 0.001, 0, 0).scale).toBe(0.1);
  });
});

describe("panBy / converters", () => {
  it("translates the transform", () => {
    expect(panBy({ x: 5, y: 5, scale: 2 }, 10, -3)).toEqual({ x: 15, y: 2, scale: 2 });
  });

  it("round-trips world↔screen", () => {
    const t = { x: 30, y: -10, scale: 2 };
    const s = worldToScreen(t, 12, 8);
    expect(screenToWorld(t, s.x, s.y)).toEqual({ x: 12, y: 8 });
  });
});

describe("clampScale", () => {
  it("bounds to [0.1, 4]", () => {
    expect(clampScale(0)).toBe(0.1);
    expect(clampScale(9)).toBe(4);
  });
});
