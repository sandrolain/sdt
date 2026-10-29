// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import {
  clampPetHeight,
  DEFAULT_PET_HEIGHT,
  MAX_PET_HEIGHT,
  MIN_PET_HEIGHT,
  PET_HEIGHT_KEY,
  readPetHeight,
  writePetHeight,
} from "./petHeight";

afterEach(() => {
  localStorage.clear();
});

describe("clampPetHeight", () => {
  it("holds a height between the band's floor and its ceiling", () => {
    expect(clampPetHeight(MIN_PET_HEIGHT - 200)).toBe(MIN_PET_HEIGHT);
    expect(clampPetHeight(MAX_PET_HEIGHT + 400)).toBe(MAX_PET_HEIGHT);
    expect(clampPetHeight(213.4)).toBe(213);
  });

  it("falls back to the default when the value is not a number", () => {
    expect(clampPetHeight(Number.NaN)).toBe(DEFAULT_PET_HEIGHT);
    expect(clampPetHeight(Number.POSITIVE_INFINITY)).toBe(DEFAULT_PET_HEIGHT);
  });
});

describe("the remembered height", () => {
  it("is the default until the reader has resized the band", () => {
    expect(readPetHeight()).toBe(DEFAULT_PET_HEIGHT);
  });

  it("is the height the reader left, read back under its own key", () => {
    writePetHeight(213);
    expect(localStorage.getItem(PET_HEIGHT_KEY)).toBe("213");
    expect(readPetHeight()).toBe(213);
  });

  it("clamps on the way in, so a height out of range is never stored as it stands", () => {
    writePetHeight(MAX_PET_HEIGHT + 400);
    expect(localStorage.getItem(PET_HEIGHT_KEY)).toBe(String(MAX_PET_HEIGHT));
  });

  it("reads a value it did not write as the default rather than as a broken band", () => {
    localStorage.setItem(PET_HEIGHT_KEY, "tall");
    expect(readPetHeight()).toBe(DEFAULT_PET_HEIGHT);
  });
});
