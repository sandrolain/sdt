import { describe, expect, it } from "vitest";
import { PET_2D_VARIANTS } from "./pet/types";
import { PET_ONCE, petTimingFor } from "./petTimings";

describe("PET_ONCE", () => {
  it("keeps the WebGL three on the library's durations", () => {
    for (const variant of ["dot", "sciame", "trama"] as const) {
      expect(PET_ONCE[variant]).toEqual({ successMs: 1_800, errorMs: 1_300 });
    }
  });

  it("gives Scriptor the time its word needs", () => {
    expect(PET_ONCE.scriptor).toEqual({ successMs: 2_600, errorMs: 2_000 });
  });

  it("covers every 2D variant with the same ending", () => {
    for (const variant of PET_2D_VARIANTS) {
      expect(petTimingFor(variant)).toEqual(
        variant === "scriptor"
          ? { successMs: 2_600, errorMs: 2_000 }
          : { successMs: 2_400, errorMs: 1_800 },
      );
    }
  });
});
