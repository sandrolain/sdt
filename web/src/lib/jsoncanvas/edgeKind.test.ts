import { describe, expect, it } from "vitest";
import { edgeKindStyle } from "./edgeKind";

describe("edgeKindStyle", () => {
  it("styles a relation (heavier) and a link (dashed)", () => {
    expect(edgeKindStyle("relation").width).toBeGreaterThan(0);
    expect(edgeKindStyle("link").dash).toBeTruthy();
  });

  it("degrades an unknown or absent kind to neutral", () => {
    expect(edgeKindStyle("weird")).toEqual({});
    expect(edgeKindStyle(undefined)).toEqual({});
    expect(edgeKindStyle("")).toEqual({});
  });
});
