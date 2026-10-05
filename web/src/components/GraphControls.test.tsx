// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphControls } from "./GraphControls";
import { initialGraphTools } from "../lib/graphTools";

afterEach(cleanup);

function renderControls(overrides: Record<string, unknown> = {}) {
  const props = {
    tools: initialGraphTools,
    allVerbs: ["depends_on", "refers_to"],
    allKinds: ["link", "relation"],
    clusters: [{ id: "analysis", color: "#cba6f7", count: 3 }],
    selectedId: null,
    selectedTitle: null,
    nodeOptions: [{ id: "a", label: "Alpha" }],
    pathFrom: "",
    pathTo: "",
    pathActive: false,
    onTools: vi.fn(),
    onPathFrom: vi.fn(),
    onPathTo: vi.fn(),
    onFindPath: vi.fn(),
    onClearPath: vi.fn(),
    onFit: vi.fn(),
    onClear: vi.fn(),
    onOpen: vi.fn(),
    onExportSVG: vi.fn(),
    onExportPNG: vi.fn(),
    ...overrides,
  };
  render(<GraphControls {...props} />);
  return props;
}

describe("GraphControls", () => {
  it("dispatches the mode switch", async () => {
    const props = renderControls();
    await userEvent.click(screen.getByRole("button", { name: /3D/ }));
    expect(props.onTools).toHaveBeenCalledWith({ type: "mode", value: "3d" });
  });

  it("runs fit and export from the action row", async () => {
    const props = renderControls();
    await userEvent.click(screen.getByRole("button", { name: "Fit" }));
    await userEvent.click(screen.getByRole("button", { name: "SVG" }));
    expect(props.onFit).toHaveBeenCalled();
    expect(props.onExportSVG).toHaveBeenCalled();
  });

  it("shows the selected node and opens it", async () => {
    const props = renderControls({ selectedId: "a", selectedTitle: "Alpha" });
    await userEvent.click(screen.getByRole("button", { name: "Open page" }));
    expect(props.onOpen).toHaveBeenCalledWith("a");
  });

  it("disables the path finder until both ends are chosen", () => {
    renderControls();
    expect((screen.getByRole("button", { name: "Find" }) as HTMLButtonElement).disabled).toBe(true);
    cleanup();
    renderControls({ pathFrom: "a", pathTo: "b" });
    expect((screen.getByRole("button", { name: "Find" }) as HTMLButtonElement).disabled).toBe(
      false,
    );
  });
});
