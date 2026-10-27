// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { PetCore } from "./core";
import type { PetState } from "./types";

/** A pet with no renderer: the core's clock is what these tests read, not the pixels. */
class FakeCore extends PetCore {
  protected readonly once = { success: 0.01, error: 0.01 };
  paints: number[] = [];
  snaps = 0;
  disposed = 0;

  protected paint(dt: number): void {
    this.paints.push(dt);
  }

  protected snap(): void {
    this.snaps++;
  }

  go(): void {
    this.start();
  }

  get energyValue(): number {
    return this.energy;
  }

  get lookValue(): { x: number; y: number } {
    return { ...this.look };
  }

  protected onDispose(): void {
    this.disposed++;
  }
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("PetCore", () => {
  it("notifies listeners on a real change only", () => {
    const pet = new FakeCore();
    const seen: PetState[] = [];
    pet.on("stateChange", (s) => seen.push(s));

    pet.setState("working");
    pet.setState("working");
    pet.setState("listening");

    expect(seen).toEqual(["working", "listening"]);
    expect(pet.state).toBe("listening");
  });

  it("paints one settled frame per state change while still", () => {
    const pet = new FakeCore();
    pet.setStill(true);
    expect(pet.snaps).toBe(1);
    expect(pet.paints).toEqual([0]);

    pet.setState("thinking");
    expect(pet.snaps).toBe(2);
    expect(pet.paints).toEqual([0, 0]);
  });

  it("caps the energy and clamps the gaze", () => {
    const pet = new FakeCore();
    pet.pulse(2);
    expect(pet.energyValue).toBe(1.5);
    pet.lookAt(3, -3);
    expect(pet.lookValue).toEqual({ x: 1, y: -1 });
  });

  it("ignores a zero-size resize and repaints a still pet on a real one", () => {
    const pet = new FakeCore();
    pet.setStill(true);
    pet.paints = [];
    pet.resize(0, 0);
    expect(pet.paints).toEqual([]);
    pet.resize(80, 40);
    expect(pet.paints).toEqual([0]);
  });

  it("returns a one-shot state to idle from the loop", () => {
    const frames: FrameRequestCallback[] = [];
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
      frames.push(cb);
      return frames.length;
    });
    vi.stubGlobal("cancelAnimationFrame", () => {});

    const pet = new FakeCore();
    const seen: PetState[] = [];
    pet.on("stateChange", (s) => seen.push(s));
    pet.go();
    pet.setState("success");
    const frame = frames.shift();
    expect(frame).toBeDefined();
    frame?.(performance.now() + 100);

    expect(seen).toEqual(["success", "idle"]);
  });

  it("clears listeners and releases the family on dispose", () => {
    const pet = new FakeCore();
    const seen: PetState[] = [];
    pet.on("stateChange", (s) => seen.push(s));
    pet.dispose();
    pet.setState("working");
    expect(seen).toEqual([]);
    expect(pet.disposed).toBe(1);
  });
});
