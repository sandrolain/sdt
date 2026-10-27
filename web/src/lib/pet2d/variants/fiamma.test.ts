// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { PET_STATES } from "../../pet/types";
import { FiammaPet } from "./fiamma";
import { fake2dCanvas } from "../testFake";

describe("FiammaPet", () => {
  it("derives the glyph cell from density", () => {
    const dense = new FiammaPet(fake2dCanvas().canvas, { density: 1 });
    const sparse = new FiammaPet(fake2dCanvas().canvas, { density: 0.6 });
    expect(dense.cell).toBe(12);
    expect(sparse.cell).toBe(20);
  });

  it("paints every state as a still frame", () => {
    const fake = fake2dCanvas();
    const pet = new FiammaPet(fake.canvas);
    pet.resize(400, 176);
    pet.setStill(true);
    for (const state of PET_STATES) {
      if (pet.state === state) pet.setState(state === "idle" ? "working" : "idle");
      const before = fake.count("fillText");
      pet.setState(state);
      expect(fake.count("fillText")).toBeGreaterThan(before);
    }
  });

  it("repaints for the light theme without losing the state", () => {
    const fake = fake2dCanvas();
    const pet = new FiammaPet(fake.canvas, {}, "dark");
    pet.resize(400, 176);
    pet.setStill(true);
    pet.setState("working");
    const before = fake.count("fillText");
    pet.setTheme("light");
    expect(pet.state).toBe("working");
    expect(fake.count("fillText")).toBeGreaterThan(before);
  });

  it("keeps its own one-shot durations", () => {
    const pet = new FiammaPet(fake2dCanvas().canvas);
    const table = (pet as unknown as { once: Record<string, number> }).once;
    expect(table).toEqual({ success: 2.4, error: 1.8 });
  });
});
