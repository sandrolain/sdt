// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { PET_STATES } from "../../pet/types";
import { PossibilePet } from "./possibile";
import { fake2dCanvas } from "../testFake";

describe("PossibilePet", () => {
  it("paints every state as a still frame", () => {
    const fake = fake2dCanvas();
    const pet = new PossibilePet(fake.canvas);
    pet.resize(400, 176);
    pet.setStill(true);
    for (const state of PET_STATES) {
      if (pet.state === state) pet.setState(state === "idle" ? "working" : "idle");
      const before = fake.count("stroke");
      pet.setState(state);
      expect(fake.count("stroke")).toBeGreaterThan(before);
    }
  });

  it("keeps its own one-shot durations", () => {
    const pet = new PossibilePet(fake2dCanvas().canvas);
    const table = (pet as unknown as { once: Record<string, number> }).once;
    expect(table).toEqual({ success: 2.4, error: 1.8 });
  });

  it("repaints for the light theme and keeps the state", () => {
    const fake = fake2dCanvas();
    const pet = new PossibilePet(fake.canvas, {}, "dark");
    pet.resize(400, 176);
    pet.setStill(true);
    pet.setState("thinking");
    const before = fake.count("stroke");
    pet.setTheme("light");
    expect(pet.state).toBe("thinking");
    expect(fake.count("stroke")).toBeGreaterThan(before);
  });

  it("ignores density: it has no particle budget to scale", () => {
    const a = fake2dCanvas();
    const b = fake2dCanvas();
    const dense = new PossibilePet(a.canvas, { density: 1 });
    const sparse = new PossibilePet(b.canvas, { density: 0.35 });
    for (const pet of [dense, sparse]) {
      pet.resize(400, 176);
      pet.setStill(true);
    }
    expect(a.count("stroke")).toBe(b.count("stroke"));
  });
});
