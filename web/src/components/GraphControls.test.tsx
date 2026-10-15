// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphControls } from "./GraphControls";
import { initialGraphTools } from "../lib/graphTools";
import { fetchDoc } from "../lib/api";

// The guide reads a corpus page through the viewer's own document endpoint; the
// fetch is stubbed so the test owns the content.
vi.mock("../lib/api", () => ({
  fetchDoc: vi.fn(async (path: string) => ({
    path,
    frontmatter: "",
    markdown: "## Summary\n\nThe map draws semantic proximity.",
  })),
}));

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
  it("hides the semantic mode control when no docs-map payload is present", () => {
    renderControls();
    expect(screen.queryByRole("group", { name: "Semantic mode" })).toBeNull();
  });

  it("dispatches the semantic mode switch when the control is present", async () => {
    const onChange = vi.fn();
    renderControls({ semantic: { mode: "edges", onChange } });
    // Edges is the default and is the pressed option
    expect(screen.getByRole("button", { name: "Edges" }).getAttribute("aria-pressed")).toBe("true");
    await userEvent.click(screen.getByRole("button", { name: "Projection" }));
    expect(onChange).toHaveBeenCalledWith("projection");
  });

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
    const toggle = screen.getByRole("button", { name: "Surface" });
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

  // The docs map's capability map (analysis 20261013-063009, D1): a control that
  // cannot change the current mode is disabled with its reason, and one that can
  // never act is absent rather than greyed.
  describe("docs-map capability", () => {
    const docsMap = (mode: "edges" | "projection" | "clusters" | "combined") => ({
      semantic: { mode, onChange: vi.fn() },
      allVerbs: ["semantic"],
      allKinds: [],
      clusters: [{ id: "cluster 0", color: "#cba6f7", count: 3 }],
    });

    it("names the one edge identity and drops the authored legends", () => {
      renderControls(docsMap("edges"));
      // Both the section subtitle and the row read the surface's own vocabulary.
      expect(screen.getAllByText("Semantic proximity").length).toBe(2);
      expect(
        (screen.getByRole("checkbox", { name: "Semantic proximity" }) as HTMLInputElement).disabled,
      ).toBe(false);
      expect(screen.queryByRole("checkbox", { name: "link" })).toBeNull();
      expect(screen.queryByText("Edge kinds")).toBeNull();
      expect(screen.queryByText("Cluster")).toBeNull();
    });

    it("disables Layout and Explore in Projection, and says why", () => {
      renderControls(docsMap("projection"));
      expect(screen.getByLabelText("Graph mode")).toBeTruthy();
      expect(screen.getAllByText("Positions come from the projection in this mode").length).toBe(1);
      expect(screen.getAllByText("Projection keeps no links to explore").length).toBe(1);
      expect(
        (screen.getByRole("switch", { name: "Hub" }) as HTMLInputElement).disabled ??
          screen.getByRole("switch", { name: "Hub" }).getAttribute("data-disabled"),
      ).toBeTruthy();
      // The path finder has no path to find: both ends and Clear are inert too.
      expect((screen.getByRole("button", { name: "Find" }) as HTMLButtonElement).disabled).toBe(
        true,
      );
    });

    it("keeps every control it can use live in Edges", () => {
      renderControls(docsMap("edges"));
      expect(screen.queryByText("Positions come from the projection in this mode")).toBeNull();
      expect(screen.queryByText("Projection keeps no links to explore")).toBeNull();
      expect(
        screen.getByRole("checkbox", { name: "Semantic proximity" }) as HTMLInputElement,
      ).toBeTruthy();
    });

    it("leaves the wiki graph's own legends and cluster key in place", () => {
      renderControls();
      expect(screen.getByText("Cluster")).toBeTruthy();
      expect(screen.getByText("Relations")).toBeTruthy();
      expect(screen.getByText("Edge kinds")).toBeTruthy();
      expect(screen.getByRole("checkbox", { name: "link" })).toBeTruthy();
    });
  });

  it("reports the hovered legend row and dims its siblings", async () => {
    const onHoverGroup = vi.fn();
    const props = renderControls({
      hoverGroup: "analysis",
      onHoverGroup,
      clusters: [
        { id: "analysis", color: "#cba6f7", count: 3 },
        { id: "notes", color: "#94e2d5", count: 2 },
      ],
    });
    // The hovered row stays legible; the others are dimmed as rows.
    expect(
      screen.getByRole("checkbox", { name: "notes (2)" }).closest("label")?.className,
    ).toContain("is-dimmed");
    expect(
      screen.getByRole("checkbox", { name: "analysis (3)" }).closest("label")?.className,
    ).not.toContain("is-dimmed");

    await userEvent.hover(screen.getByRole("checkbox", { name: "notes (2)" }));
    expect(onHoverGroup).toHaveBeenCalledWith("notes");
    await userEvent.unhover(screen.getByRole("checkbox", { name: "notes (2)" }));
    expect(onHoverGroup).toHaveBeenCalledWith(null);
    expect(props.onTools).not.toHaveBeenCalled();
  });

  describe("inline guide", () => {
    it("is offered on the docs map, off by default, and reveals one hotspot per section", async () => {
      renderControls({ semantic: { mode: "edges", onChange: vi.fn() }, allVerbs: ["semantic"] });
      const guide = screen.getByRole("switch", { name: "Guide" });
      expect(guide.getAttribute("data-selected")).toBeNull();
      for (const anchor of ["surface", "filters", "explore", "actions"]) {
        expect(screen.queryByRole("button", { name: `Guide: ${anchor}` })).toBeNull();
      }

      await userEvent.click(guide);
      for (const anchor of ["surface", "filters", "explore", "actions"]) {
        expect(screen.getByRole("button", { name: `Guide: ${anchor}` })).toBeTruthy();
      }
    });

    it("opens the extended guide from a hotspot and closes it with Escape", async () => {
      renderControls({ semantic: { mode: "edges", onChange: vi.fn() }, allVerbs: ["semantic"] });
      await userEvent.click(screen.getByRole("switch", { name: "Guide" }));
      await userEvent.click(screen.getByRole("button", { name: "Guide: filters" }));

      expect(await screen.findByRole("dialog")).toBeTruthy();
      expect(await screen.findByText("The map draws semantic proximity.")).toBeTruthy();
      expect(fetchDoc).toHaveBeenCalledWith("context/wiki/viewer-docs-map-guide.md");

      await userEvent.keyboard("{Escape}");
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    });

    it("is not offered on the wiki graph, which has no map guide", () => {
      renderControls();
      expect(screen.queryByRole("switch", { name: "Guide" })).toBeNull();
    });
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
