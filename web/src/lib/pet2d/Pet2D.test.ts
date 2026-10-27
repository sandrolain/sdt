// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { Pet2D } from "./Pet2D";

type Call = { op: string; args: number[] };

function fakeContext(calls: Call[]) {
  const record =
    (op: string) =>
    (...args: unknown[]) =>
      calls.push({ op, args: args as number[] });
  return {
    setTransform: record("setTransform"),
    clearRect: record("clearRect"),
  } as unknown as CanvasRenderingContext2D;
}

class Fake2D extends Pet2D {
  protected readonly once = { success: 0.01 };
  ticks: number[] = [];
  themes = 0;
  sizes = 0;

  protected tick(dt: number): void {
    this.ticks.push(dt);
  }

  protected snap(): void {}

  get size(): { w: number; h: number } {
    return { w: this.canvas.width, h: this.canvas.height };
  }

  protected onTheme(): void {
    this.themes++;
  }

  protected onSize(): void {
    this.sizes++;
  }

  go(): void {
    this.start();
  }
}

function host(calls: Call[]): HTMLCanvasElement {
  const canvas = document.createElement("canvas");
  canvas.getContext = (() => fakeContext(calls)) as unknown as HTMLCanvasElement["getContext"];
  return canvas;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Pet2D", () => {
  it("sizes the backing store by the capped device pixel ratio", () => {
    vi.stubGlobal("devicePixelRatio", 3);
    const calls: Call[] = [];
    const pet = new Fake2D(host(calls), { pixelRatioCap: 1.5 });
    pet.resize(120, 60);
    expect(pet.size).toEqual({ w: 180, h: 90 });
    expect(pet.sizes).toBe(1);
  });

  it("clears at the backing size and scales to CSS pixels before drawing", () => {
    vi.stubGlobal("devicePixelRatio", 2);
    const calls: Call[] = [];
    const pet = new Fake2D(host(calls), { pixelRatioCap: 2 });
    pet.setStill(true);
    const ops = calls.map((c) => `${c.op}(${c.args.join(",")})`);
    expect(ops).toContain("setTransform(1,0,0,1,0,0)");
    expect(ops).toContain("setTransform(2,0,0,2,0,0)");
    expect(pet.ticks).toEqual([0]);
  });

  it("repaints a still pet on a theme change and counts the ink derivation", () => {
    const calls: Call[] = [];
    const pet = new Fake2D(host(calls));
    pet.setStill(true);
    pet.ticks = [];
    pet.setTheme("light");
    expect(pet.themes).toBe(1);
    expect(pet.ticks).toEqual([0]);
    pet.setTheme("light");
    expect(pet.themes).toBe(1);
  });

  it("throws when the canvas cannot give a 2D context", () => {
    const canvas = document.createElement("canvas");
    canvas.getContext = (() => null) as unknown as HTMLCanvasElement["getContext"];
    expect(() => new Fake2D(canvas)).toThrow(/2D context/);
  });
});
