// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { BoardView } from "./BoardView";
import { normalizeBoard } from "../lib/canvas";

const flow = vi.hoisted(() => ({
  zoomIn: vi.fn(),
  zoomOut: vi.fn(),
  fitView: vi.fn(),
}));

// React Flow relies on real layout/d3 event wiring that jsdom cannot provide;
// the mock renders each node through its registered nodeType and each edge
// label, so the board wiring, adapter and node component are still exercised.
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
    MarkerType: { ArrowClosed: "arrowclosed" },
    Position: { Left: "left", Right: "right" },
    Handle: () => null,
    Background: () => null,
    Controls: () => null,
    ReactFlowProvider: ({ children }: { children?: unknown }) =>
      React.createElement(Fragment, null, children as never),
    useReactFlow: () => flow,
    ReactFlow: (props: AnyProps) =>
      React.createElement(
        "div",
        { "data-testid": "flow" },
        (props.nodes ?? []).map((node) => {
          const Component = props.nodeTypes?.[node.type ?? "text"];
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

const MODEL = normalizeBoard({
  nodes: [
    { id: "a", type: "text", x: 0, y: 0, width: 120, height: 60, text: "Alpha" },
    { id: "b", type: "text", x: 220, y: 0, width: 120, height: 60, text: "Beta" },
  ],
  edges: [{ id: "e0", fromNode: "a", toNode: "b", label: "refers_to" }],
});

function renderBoard(onOpen = vi.fn()) {
  render(
    <MemoryRouter>
      <BoardView model={MODEL} onOpen={onOpen} />
    </MemoryRouter>,
  );
  return onOpen;
}

afterEach(cleanup);

describe("BoardView", () => {
  it("renders cards and relation edges", () => {
    renderBoard();
    expect(screen.getByRole("button", { name: "Alpha" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Beta" })).toBeTruthy();
    expect(screen.getByText("refers_to")).toBeTruthy();
    expect(screen.getByRole("application", { name: /read-only/ })).toBeTruthy();
  });

  it("opens a card through the callback", async () => {
    const onOpen = renderBoard();
    await userEvent.click(screen.getByRole("button", { name: "Alpha" }));
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: "a" }));
  });

  it("exposes zoom controls wired to the viewport", async () => {
    renderBoard();
    await userEvent.click(screen.getByRole("button", { name: "Zoom in" }));
    await userEvent.click(screen.getByRole("button", { name: "Zoom out" }));
    await userEvent.click(screen.getByRole("button", { name: "Fit" }));
    expect(flow.zoomIn).toHaveBeenCalled();
    expect(flow.zoomOut).toHaveBeenCalled();
    expect(flow.fitView).toHaveBeenCalled();
    expect(screen.getByRole("group", { name: "Board zoom" })).toBeTruthy();
  });
});
