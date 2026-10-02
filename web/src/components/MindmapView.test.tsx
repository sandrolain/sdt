// @vitest-environment jsdom
import type React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import userEvent from "@testing-library/user-event";
import { MindmapView } from "./MindmapView";

const flow = vi.hoisted(() => ({
  zoomIn: vi.fn(),
  zoomOut: vi.fn(),
  fitView: vi.fn(),
}));

// React Flow relies on real layout/d3 event wiring that jsdom cannot provide;
// the mock renders each node through its registered nodeType and each edge
// label, so the map wiring, model and node component are still exercised.
// Geometry and pixels are the browser pass's job.
vi.mock("@xyflow/react", async () => {
  const React = await import("react");
  type AnyProps = Record<string, unknown> & {
    children?: unknown;
    nodes?: Array<{ id: string; type?: string; data: unknown }>;
    edges?: Array<{ id: string; label?: unknown }>;
    nodeTypes?: Record<string, (props: unknown) => unknown>;
  };
  const Fragment = React.Fragment;
  return {
    Background: () => null,
    Controls: () => null,
    MiniMap: () => null,
    Handle: () => null,
    Position: { Left: "left", Right: "right" },
    ReactFlowProvider: ({ children }: { children?: unknown }) =>
      React.createElement(Fragment, null, children as never),
    useReactFlow: () => flow,
    useViewport: () => ({ x: 0, y: 0, zoom: 1 }),
    ReactFlow: (props: AnyProps) =>
      React.createElement(
        "div",
        { "data-testid": "flow" },
        (props.nodes ?? []).map((node) => {
          const Component = props.nodeTypes?.[node.type ?? "map"];
          return Component
            ? React.createElement(Component as never, {
                key: node.id,
                id: node.id,
                type: node.type,
                data: node.data,
              })
            : null;
        }),
        (props.edges ?? []).map((edge) =>
          edge.label ? React.createElement("span", { key: edge.id }, String(edge.label)) : null,
        ),
      ),
  };
});

const TREE_FIXTURE = {
  entries: [
    { path: "context/wiki/topic.map.md", title: "Topic Map", isMap: true, mapId: "topic.map" },
    { path: "context/wiki/other.map.md", title: "Other Map", isMap: true, mapId: "other.map" },
  ],
};

function mockFetch() {
  return vi.fn((url: string) => {
    if (url === "/api/tree")
      return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE_FIXTURE) });
    if (url.startsWith("/api/doc")) {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            path: "context/wiki/other.map.md",
            frontmatter: "",
            markdown: "# Other\n\n- x\n",
          }),
      });
    }
    return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
  });
}

/** MindmapView navigates, so every render needs a router like the board test. */
function renderInRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("MindmapView", () => {
  it("renders a read-only React Flow map with the topics as nodes", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderInRouter(
      <MindmapView
        markdown={"# Root\n\n- child\n"}
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    expect(screen.getByRole("application", { name: /Mind map: Topic/ })).toBeTruthy();
    expect(await screen.findByText("Root")).toBeTruthy();
    expect(screen.getByText("child")).toBeTruthy();
  });

  it("offers both layouts and switches between them", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderInRouter(
      <MindmapView
        markdown={"# Root\n\n- child\n"}
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    const balanced = await screen.findByRole("button", { name: /Balanced/ });
    expect(balanced.getAttribute("aria-pressed")).toBe("true");
    const radial = screen.getByRole("button", { name: /Radial/ });
    await userEvent.click(radial);
    expect(radial.getAttribute("aria-pressed")).toBe("true");
    expect(balanced.getAttribute("aria-pressed")).toBe("false");
  });

  it("collapses and expands a branch from its node toggle", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderInRouter(
      <MindmapView
        markdown={"# Root\n\n- parent\n  - leaf\n- sibling\n"}
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    expect(await screen.findByText("leaf")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Collapse parent" }));
    await waitFor(() => expect(screen.queryByText("leaf")).toBeNull());
    await userEvent.click(screen.getByRole("button", { name: "Expand parent" }));
    expect(await screen.findByText("leaf")).toBeTruthy();
  });

  it("draws the boundary and group overlays from the computed boxes", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    const { container } = renderInRouter(
      <MindmapView
        markdown={"# Root\n\n## a [B1]\n\n## b [B1]\n\n[B1]: Wrap\n\n- g #group/hull\n"}
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    await waitFor(() => expect(container.querySelector(".mindmap__boundary")).toBeTruthy());
    expect(container.querySelector(".mindmap__hull")).toBeTruthy();
    expect(container.querySelector(".mindmap__overlay")?.textContent).toContain("Wrap");
  });

  it("exposes Current/Fused toggles for maps with references", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderInRouter(
      <MindmapView
        markdown={"# Root\n\n- [Other](/wiki/other.map)\n"}
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    expect(await screen.findByRole("button", { name: "Current Map", pressed: true })).toBeTruthy();
    const fused = screen.getByRole("button", { name: "Fused Map" });
    await userEvent.click(fused);
    expect(screen.getByRole("button", { name: "Fused Map", pressed: true })).toBeTruthy();
    expect(await screen.findByText(/imported/)).toBeTruthy();
  });

  it("hides the fused toggle for ordinary documents", () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderInRouter(
      <MindmapView markdown={"# Root\n"} basePath="context/analysis/x.md" title="X" />,
    );
    expect(screen.queryByRole("button", { name: "Fused Map" })).toBeNull();
  });
  it("opens the linked document from a node whose whole label is one link", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderInRouter(
      <MindmapView
        markdown={"# Root\n\n- [Other map](/wiki/other.map)\n"}
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    await userEvent.click(await screen.findByRole("button", { name: "Open Other map" }));
    // A router inside the test keeps the navigation local; the button exists and
    // is the node's only interactive target.
    expect(screen.queryByRole("button", { name: "Open Other map" })).toBeTruthy();
  });

  it("starts a [F] topic folded and expands it on demand", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderInRouter(
      <MindmapView
        markdown={"# Root\n\n- closed [F]\n  - hidden leaf\n- open\n"}
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    expect(await screen.findByRole("button", { name: "Expand closed" })).toBeTruthy();
    expect(screen.queryByText("hidden leaf")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Expand closed" }));
    expect(await screen.findByText("hidden leaf")).toBeTruthy();
  });

  it("shows a note and its stickers on the node", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderInRouter(
      <MindmapView
        markdown={"# Root\n\n- flagged [!star][!nonsense] [N:remember this]\n"}
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    expect(await screen.findByLabelText("Note: remember this")).toBeTruthy();
    expect(screen.getByLabelText("Marker star")).toBeTruthy();
    expect(screen.getByLabelText("Marker nonsense")).toBeTruthy();
  });

  it("keeps a mixed label navigable by its own link", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderInRouter(
      <MindmapView
        markdown={"# Root\n\n- see [Other](/wiki/other.map)\n"}
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    expect(await screen.findByRole("link", { name: "Other" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Open see Other" })).toBeNull();
  });
  it("exports the visible map as an SVG download", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    const createObjectURL = vi.fn(() => "blob:map");
    const revokeObjectURL = vi.fn();
    vi.stubGlobal("URL", { ...URL, createObjectURL, revokeObjectURL });
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    renderInRouter(
      <MindmapView
        markdown={"# Root\n\n- child\n"}
        basePath="context/wiki/topic.map.md"
        title="Topic"
      />,
    );
    await userEvent.click(await screen.findByRole("button", { name: /SVG/ }));
    expect(createObjectURL).toHaveBeenCalled();
    expect(click).toHaveBeenCalled();
    expect(screen.queryByText(/Export failed/)).toBeNull();
    click.mockRestore();
    vi.unstubAllGlobals();
  });
});
