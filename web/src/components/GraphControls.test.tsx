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

  it("toggles each legend dimension and resets the filters", async () => {
    const props = renderControls();
    await userEvent.click(screen.getByRole("checkbox", { name: /analysis/ }));
    expect(props.onTools).toHaveBeenCalledWith({ type: "toggleGroup", value: "analysis" });
    await userEvent.click(screen.getByRole("checkbox", { name: "depends_on" }));
    expect(props.onTools).toHaveBeenCalledWith({ type: "toggleRelation", value: "depends_on" });
    await userEvent.click(screen.getByRole("checkbox", { name: "link" }));
    expect(props.onTools).toHaveBeenCalledWith({ type: "toggleKind", value: "link" });
    await userEvent.click(screen.getByRole("button", { name: "Reset" }));
    expect(props.onTools).toHaveBeenCalledWith({ type: "clearFilters" });
  });

  it("collapses a section while keeping its header", async () => {
    renderControls();
    const toggle = screen.getByRole("button", { name: "View" });
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    await userEvent.click(toggle);
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("button", { name: /3D/ })).toBeNull();
  });

  it("names the switches and the legend toggles", () => {
    renderControls();
    expect(screen.getByRole("switch", { name: "Labels" })).toBeTruthy();
    expect(screen.getByRole("switch", { name: "Hub" })).toBeTruthy();
    expect(screen.getByRole("checkbox", { name: "analysis (3)" })).toBeTruthy();
    expect(screen.getByRole("checkbox", { name: "depends_on" })).toBeTruthy();
    expect(screen.getByRole("checkbox", { name: "link" })).toBeTruthy();
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
