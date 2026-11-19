import { describe, expect, it } from "vitest";
import { planScopeLabel } from "./delegation";

describe("planScopeLabel", () => {
  it("labels a task linked to the whole plan", () => {
    expect(planScopeLabel()).toBe("Entire plan");
    expect(planScopeLabel("  ")).toBe("Entire plan");
  });

  it("preserves the linked phase", () => {
    expect(planScopeLabel("c2")).toBe("c2");
  });
});
