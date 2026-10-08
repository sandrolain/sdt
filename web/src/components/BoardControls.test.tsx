// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BoardControls } from "./BoardControls";

afterEach(cleanup);

function renderControls(overrides: Record<string, unknown> = {}) {
  const props = {
    sources: [{ path: "context/wiki/x.canvas", label: "X" }],
    source: "",
    onSource: vi.fn(),
    crumbs: [
      { id: "", label: "Wiki graph" },
      { id: "n1", label: "Level one" },
    ],
    onCrumb: vi.fn(),
    zoom: 1,
    onZoomIn: vi.fn(),
    onZoomOut: vi.fn(),
    onFit: vi.fn(),
    mode: "2d" as const,
    onMode: vi.fn(),
    layers: [
      { id: 0, name: "Level 0" },
      { id: 1, name: "Deep" },
    ],
    hiddenLayers: [] as number[],
    onToggleLayer: vi.fn(),
    showMinimap: true,
    onShowMinimap: vi.fn(),
    ...overrides,
  };
  render(<BoardControls {...props} />);
  return props;
}

describe("BoardControls", () => {
  it("dispatches the mode switch", async () => {
    const props = renderControls();
    await userEvent.click(screen.getByRole("button", { name: /3D/ }));
    expect(props.onMode).toHaveBeenCalledWith("3d");
  });

  it("runs the zoom and fit controls", async () => {
    const props = renderControls();
    await userEvent.click(screen.getByRole("button", { name: "In" }));
    await userEvent.click(screen.getByRole("button", { name: "Out" }));
    await userEvent.click(screen.getByRole("button", { name: "Fit" }));
    expect(props.onZoomIn).toHaveBeenCalled();
    expect(props.onZoomOut).toHaveBeenCalled();
    expect(props.onFit).toHaveBeenCalled();
  });

  it("toggles the minimap and a layer", async () => {
    const props = renderControls({ mode: "3d" });
    await userEvent.click(screen.getByRole("switch", { name: "Minimap" }));
    await userEvent.click(screen.getByRole("switch", { name: "Deep" }));
    expect(props.onShowMinimap).toHaveBeenCalledWith(false);
    expect(props.onToggleLayer).toHaveBeenCalledWith(1);
  });

  it("hides the Layers section in 2D and shows it in 3D", () => {
    renderControls({ mode: "2d" });
    expect(screen.queryByRole("switch", { name: "Deep" })).toBeNull();
    cleanup();
    renderControls({ mode: "3d" });
    expect(screen.getByRole("switch", { name: "Deep" })).toBeTruthy();
  });

  it("pops a drill-down level from the breadcrumb", async () => {
    const props = renderControls();
    await userEvent.click(screen.getByRole("button", { name: "Wiki graph" }));
    expect(props.onCrumb).toHaveBeenCalledWith("");
  });
});
