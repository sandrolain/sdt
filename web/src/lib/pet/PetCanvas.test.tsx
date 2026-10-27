// @vitest-environment jsdom
import { cleanup, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PetCanvas } from "./PetCanvas";

/**
 * The factories are mocked because both create a real context: what the host owes is the
 * family resolution (a 2D choice must never reach the WebGL factory), the theme hand-off and
 * the teardown, and those are visible without a pixel.
 */
const h = vi.hoisted(() => {
  const calls = { two: 0, webgl: 0, themes: [] as string[], disposed: 0 };
  const controller = () => ({
    state: "idle" as const,
    setState: () => {},
    pulse: () => {},
    lookAt: () => {},
    setPaused: () => {},
    setStill: () => {},
    resize: () => {},
    on: () => () => {},
    dispose: () => {
      calls.disposed++;
    },
    setTheme: (theme: string) => {
      calls.themes.push(theme);
    },
  });
  return { calls, controller };
});

vi.mock("../pet2d/createPet", () => ({
  createPet2D: (_variant: string, _canvas: unknown, _options: unknown, theme: string) => {
    h.calls.two++;
    const pet = h.controller();
    pet.setTheme(theme);
    return pet;
  },
}));

vi.mock("./createPet", () => ({
  createPet: () => {
    h.calls.webgl++;
    return h.controller();
  },
}));

beforeEach(() => {
  h.calls.two = 0;
  h.calls.webgl = 0;
  h.calls.themes = [];
  h.calls.disposed = 0;
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  vi.stubGlobal(
    "IntersectionObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("PetCanvas", () => {
  it("resolves the 2D factory for a 2D family and never the WebGL one", async () => {
    render(<PetCanvas family="2d" variant="fiamma" theme="dark" />);
    await waitFor(() => expect(h.calls.two).toBe(1));
    expect(h.calls.webgl).toBe(0);
    expect(h.calls.themes).toContain("dark");
  });

  it("hands a theme change to a controller that repaints", async () => {
    const { rerender } = render(<PetCanvas family="2d" variant="scriptor" theme="dark" />);
    await waitFor(() => expect(h.calls.two).toBe(1));
    rerender(<PetCanvas family="2d" variant="scriptor" theme="light" />);
    await waitFor(() => expect(h.calls.themes).toContain("light"));
  });

  it("resolves the WebGL factory for the default family", async () => {
    render(<PetCanvas variant="dot" />);
    await waitFor(() => expect(h.calls.webgl).toBe(1));
    expect(h.calls.two).toBe(0);
  });

  it("disposes a pet whose mount was cancelled before the factory resolved", async () => {
    const { unmount } = render(<PetCanvas family="2d" variant="possibile" />);
    unmount();
    await waitFor(() => expect(h.calls.disposed).toBeGreaterThanOrEqual(1));
  });
});
