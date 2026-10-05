import { describe, expect, it } from "vitest";
import { buildColorMaps, DEFAULT_GROUP, DEFAULT_PALETTE, toRGB } from "./colors";

describe("buildColorMaps", () => {
  it("assigns palette colours in first-appearance order", () => {
    const maps = buildColorMaps(
      [
        { id: "a", group: "x" },
        { id: "b", group: "y" },
        { id: "c", group: "x" },
      ],
      [{ source: "a", target: "b", type: "depends_on" }],
    );
    expect(maps.groups).toEqual([
      ["x", DEFAULT_PALETTE[0]],
      ["y", DEFAULT_PALETTE[1]],
    ]);
    expect(maps.relations).toEqual([["depends_on", expect.any(String)]]);
  });

  it("lets supplied colours win and falls back to the default group", () => {
    const maps = buildColorMaps([{ id: "a" }], [], { [DEFAULT_GROUP]: "#123456" });
    expect(maps.groups).toEqual([[DEFAULT_GROUP, "#123456"]]);
  });

  it("ignores links without a type and rejects empty inputs", () => {
    expect(buildColorMaps([{ id: "a" }], [{ source: "a", target: "b" }]).relations).toEqual([]);
    expect(() => buildColorMaps("bad" as never, [])).toThrow(TypeError);
    expect(() => buildColorMaps([], [], {}, {}, [])).toThrow(TypeError);
  });
});

describe("toRGB", () => {
  it("returns the sRGB components of a hex colour", () => {
    const [r, g, b] = toRGB("#ff0000");
    expect(r).toBeCloseTo(1, 3);
    expect(g).toBeCloseTo(0, 3);
    expect(b).toBeCloseTo(0, 3);
  });

  it("falls back to the neutral colour for an empty value", () => {
    expect(toRGB("")).toBeDefined();
  });
});
