// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import {
  PET_ENABLED_KEY,
  PET_VARIANT_KEY,
  effectiveVariant,
  readPetEnabled,
  readPetVariant,
  writePetEnabled,
  writePetVariant,
} from "./petPreference";

afterEach(() => {
  localStorage.clear();
});

describe("the pet's on/off preference", () => {
  it("is on when nothing is remembered", () => {
    expect(readPetEnabled()).toBe(true);
  });

  it("remembers the reader's off, and forgets it when turned back on", () => {
    writePetEnabled(false);
    expect(localStorage.getItem(PET_ENABLED_KEY)).toBe("off");
    expect(readPetEnabled()).toBe(false);
    writePetEnabled(true);
    expect(localStorage.getItem(PET_ENABLED_KEY)).toBeNull();
    expect(readPetEnabled()).toBe(true);
  });

  it("reads anything that is not the off marker as on", () => {
    localStorage.setItem(PET_ENABLED_KEY, "nonsense");
    expect(readPetEnabled()).toBe(true);
  });
});

describe("the pet's variant preference", () => {
  it("is dot when nothing is remembered", () => {
    expect(readPetVariant()).toBe("dot");
  });

  it("round-trips every variant the pet has", () => {
    for (const variant of ["dot", "sciame", "trama", "fiamma", "scriptor", "possibile"] as const) {
      writePetVariant(variant);
      expect(localStorage.getItem(PET_VARIANT_KEY)).toBe(variant);
      expect(readPetVariant()).toBe(variant);
    }
  });

  it("falls back to dot for a value the pet does not have", () => {
    localStorage.setItem(PET_VARIANT_KEY, "dragon");
    expect(readPetVariant()).toBe("dot");
  });

  it("refuses to store a variant the pet does not have", () => {
    // @ts-expect-error — the signature is the guard; a caller reaching around it is a bug.
    writePetVariant("dragon");
    expect(localStorage.getItem(PET_VARIANT_KEY)).toBeNull();
  });
});

describe("effectiveVariant", () => {
  it("keeps the choice on the dark theme and falls back to dot on the light one", () => {
    expect(effectiveVariant("trama", "dark")).toBe("trama");
    expect(effectiveVariant("trama", "light")).toBe("dot");
    expect(effectiveVariant("dot", "light")).toBe("dot");
  });

  it("keeps a 2D character on the light theme: only the additive two fall back", () => {
    for (const variant of ["fiamma", "scriptor", "possibile"] as const) {
      expect(effectiveVariant(variant, "light")).toBe(variant);
    }
    expect(effectiveVariant("sciame", "light")).toBe("dot");
  });
});
