import { describe, expect, it } from "vitest";
import type { GNode } from "./graphModel";
import { glowColor, labelFontSize, labelSpriteSpec } from "./graphSprites";

function node(overrides: Partial<GNode> = {}): GNode {
  return {
    id: "n",
    title: "Node",
    path: "context/wiki/n.md",
    cluster: "concept",
    color: "#89b4fa",
    ...overrides,
  } as GNode;
}

describe("labelSpriteSpec", () => {
  it("shows the label when labels are on and the node is not dimmed", () => {
    const spec = labelSpriteSpec(node(), { alpha: 1, emphasis: false }, true);
    expect(spec.show).toBe(true);
    expect(spec.title).toBe("Node");
  });

  it("hides the label when the Labels tool is off", () => {
    const spec = labelSpriteSpec(node(), { alpha: 1, emphasis: false }, false);
    expect(spec.show).toBe(false);
  });

  it("dimmes labels under an active selection highlight", () => {
    const focused = labelSpriteSpec(node(), { alpha: 1, emphasis: true }, true);
    const dimmed = labelSpriteSpec(node(), { alpha: 0.18, emphasis: false }, true);
    expect(focused.show).toBe(true);
    expect(focused.alpha).toBe(1);
    expect(dimmed.show).toBe(false);
    expect(dimmed.alpha).toBe(0.18);
  });

  it("keeps the node dot visible and scaled by val even when the label is hidden", () => {
    const low = labelSpriteSpec(node({ val: 1 }), { alpha: 1, emphasis: false }, false);
    const big = labelSpriteSpec(node({ val: 16 }), { alpha: 1, emphasis: false }, false);
    expect(low.show).toBe(false);
    expect(big.radius).toBeGreaterThan(low.radius);
    expect(big.dotColor).toBe("#89b4fa");
  });
});

describe("glowColor", () => {
  it("converts a hex colour to rgba with the given alpha", () => {
    expect(glowColor("#89b4fa", 0.3)).toBe("rgba(137, 180, 250, 0.3)");
    expect(glowColor("#000000", 0)).toBe("rgba(0, 0, 0, 0)");
  });

  it("passes non-hex colours through", () => {
    expect(glowColor("var(--accent)", 0.3)).toBe("var(--accent)");
  });
});

describe("labelFontSize", () => {
  it("keeps the base size for short labels", () => {
    expect(labelFontSize("Node", 56, 28)).toBe(56);
  });

  it("shrinks long labels to fit the canvas width", () => {
    const long = "a very long document title that would overflow the sprite canvas";
    const size = labelFontSize(long, 56, 28);
    expect(size).toBeLessThan(56);
    expect(size).toBeGreaterThanOrEqual(28);
  });

  it("handles an empty label", () => {
    expect(labelFontSize("", 56)).toBe(56);
  });
});
