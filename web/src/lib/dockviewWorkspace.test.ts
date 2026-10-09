// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useDockviewWorkspace } from "./dockviewWorkspace";
import { saveLayout } from "./layoutStore";

function fakeApi() {
  return {
    fromJSON: vi.fn(),
    getPanel: vi.fn(),
    addPanel: vi.fn(),
    toJSON: vi.fn(() => ({})),
    onDidLayoutChange: vi.fn(),
  };
}

afterEach(() => localStorage.clear());

describe("useDockviewWorkspace", () => {
  it("is disabled in tests (the host renders its fallback)", () => {
    const { result } = renderHook(() => useDockviewWorkspace("k"));
    expect(result.current.enabled).toBe(false);
  });

  it("restores a stored layout, runs the host hooks and wires persistence", () => {
    saveLayout("k", { panels: 1 });
    const api = fakeApi();
    const ensureCenter = vi.fn();
    const addPanels = vi.fn();
    const extra = vi.fn();
    const { result } = renderHook(() =>
      useDockviewWorkspace("k", { ensureCenter, addPanels, onReady: extra }),
    );
    result.current.onReady({ api } as never);
    expect(result.current.apiRef.current).toBe(api);
    expect(api.fromJSON).toHaveBeenCalledWith({ panels: 1 });
    expect(ensureCenter).toHaveBeenCalledWith(api);
    expect(addPanels).toHaveBeenCalledWith(api);
    expect(extra).toHaveBeenCalledWith(api);
    expect(api.onDidLayoutChange).toHaveBeenCalled();
  });

  it("drops a corrupt stored layout and still bootstraps", () => {
    localStorage.setItem("sdt-layout:k", "{ not json");
    const api = fakeApi();
    const { result } = renderHook(() => useDockviewWorkspace("k"));
    result.current.onReady({ api } as never);
    expect(api.fromJSON).not.toHaveBeenCalled();
    expect(localStorage.getItem("sdt-layout:k")).toBeNull();
    expect(api.onDidLayoutChange).toHaveBeenCalled();
  });
});
