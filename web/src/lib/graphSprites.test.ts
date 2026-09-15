import { describe, expect, it } from "vitest";
import type { GNode } from "./graphModel";
import { labelSpriteSpec } from "./graphSprites";

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