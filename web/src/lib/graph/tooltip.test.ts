import { describe, expect, it } from "vitest";
import { edgeHoverInfo } from "./tooltip";

const labelOf = (id: string) => ({ a: "Alpha", b: "Beta" })[id] ?? id;

describe("edgeHoverInfo", () => {
  it("resolves the verb, both endpoint labels and the label", () => {
    const info = edgeHoverInfo(
      { source: "a", target: "b", type: "contains", label: "lbl" },
      labelOf,
    );
    expect(info).toEqual({ verb: "contains", fromLabel: "Alpha", toLabel: "Beta", label: "lbl" });
  });

  it("falls back to the id and a null label", () => {
    const info = edgeHoverInfo({ source: "x", target: "y" }, labelOf);
    expect(info).toEqual({ verb: "—", fromLabel: "x", toLabel: "y", label: null });
  });
});
