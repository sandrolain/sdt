// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { PET_STATES } from "../../pet/types";
import { ScriptorPet } from "./scriptor";
import { fake2dCanvas } from "../testFake";

describe("ScriptorPet", () => {
  it("derives the glyph count from density", () => {
    expect(new ScriptorPet(fake2dCanvas().canvas, { density: 1 }).budget).toBe(360);
    expect(new ScriptorPet(fake2dCanvas().canvas, { density: 0.5 }).budget).toBe(180);
  });

  it("paints every state as a still frame without a real 2D context", () => {
    const fake = fake2dCanvas();
    const pet = new ScriptorPet(fake.canvas);
    pet.resize(400, 176);
    pet.setStill(true);
    for (const state of PET_STATES) {
      if (pet.state === state) pet.setState(state === "idle" ? "working" : "idle");
      const before = fake.count("fillText");
      pet.setState(state);
      expect(fake.count("fillText")).toBeGreaterThan(before);
    }
  });

  it("falls back to a proportional advance when measureText is not available", () => {
    const fake = fake2dCanvas();
    const pet = new ScriptorPet(fake.canvas);
    pet.resize(400, 176);
    pet.setState("speaking");
    // One animated frame: the caret is drawn while the word is being written, and it can
    // only be placed with a finite advance.
    (pet as unknown as { tick(dt: number): void }).tick(0.016);
    expect(fake.count("fillRect")).toBeGreaterThan(0);
  });

  it("keeps its own one-shot durations", () => {
    const pet = new ScriptorPet(fake2dCanvas().canvas);
    const table = (pet as unknown as { once: Record<string, number> }).once;
    expect(table).toEqual({ success: 2.6, error: 2.0 });
  });

  it("repaints for the light theme", () => {
    const fake = fake2dCanvas();
    const pet = new ScriptorPet(fake.canvas, {}, "dark");
    pet.resize(400, 176);
    pet.setStill(true);
    pet.setState("thinking");
    const before = fake.count("fillText");
    pet.setTheme("light");
    expect(pet.state).toBe("thinking");
    expect(fake.count("fillText")).toBeGreaterThan(before);
  });
});
