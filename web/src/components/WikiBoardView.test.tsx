// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { WikiBoardView } from "./WikiBoardView";
import { renderNestedBody } from "./nestedCanvasBody";
import type { CanvasNode } from "../lib/jsoncanvas/document";
import { OpenDocsProvider } from "./OpenDocsProvider";

const TREE = {
  entries: [
    { path: "context/wiki/board.canvas", kind: "canvas", title: "board", canvas: true },
    { path: "context/wiki/x.md", kind: "wiki", title: "X" },
  ],
};

const DEFAULT_BOARD = {
  nodes: [{ id: "a", type: "text", x: 0, y: 0, width: 100, height: 50, text: "Alpha" }],
  edges: [],
};

const FILE_BOARD = {
  path: "context/wiki/board.canvas",
  canvas: {
    nodes: [
      {
        id: "c",
        type: "text",
        x: 0,
        y: 0,
        width: 100,
        height: 50,
        text: "From file",
        "x-layer": 2,
      },
    ],
    edges: [],
    "x-layers": [{ id: 2, name: "Deep" }],
  },
};

/** A board with an inline `nested-canvas` node carrying a child text node. */
const NESTED_BOARD = {
  nodes: [
    { id: "root", type: "text", x: 0, y: 0, width: 120, height: 40, text: "Root" },
    {
      id: "nc",
      type: "nested-canvas",
      x: 200,
      y: 0,
      width: 200,
      height: 120,
      title: "Investigation",
      canvas: {
        nodes: [{ id: "inner", type: "text", x: 0, y: 0, width: 100, height: 40, text: "Inner" }],
        edges: [],
      },
    },
  ],
  edges: [],
};

/** A board whose only node is an external `.canvas` file reference. */
const EXTERNAL_BOARD = {
  nodes: [{ id: "ext", type: "text", x: 0, y: 0, width: 120, height: 40, text: "External board" }],
  edges: [],
};

const EXTERNAL_REF_BOARD = {
  nodes: [
    {
      id: "f",
      type: "file",
      x: 0,
      y: 0,
      width: 150,
      height: 60,
      file: "context/wiki/other.canvas",
    },
  ],
  edges: [],
};

function mockFetch() {
  return vi.fn((url: string) => {
    if (url === "/api/tree")
      return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) });
    if (url.startsWith("/api/wiki/board")) {
      const isFile = url.includes("file=");
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(isFile ? FILE_BOARD : DEFAULT_BOARD),
      });
    }
    return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
  });
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function renderView() {
  return render(
    <MemoryRouter>
      <OpenDocsProvider>
        <WikiBoardView />
      </OpenDocsProvider>
    </MemoryRouter>,
  );
}

describe("WikiBoardView", () => {
  it("renders the default graph board and the docked controls", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderView();
    expect(await screen.findByRole("button", { name: "Alpha" })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Wiki graph/ })).toBeTruthy();
    expect(screen.getByRole("switch", { name: "Minimap" })).toBeTruthy();
  });

  it("switches to a selected .canvas file from the sidebar", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderView();
    await screen.findByRole("button", { name: "Alpha" });
    await userEvent.click(screen.getByRole("button", { name: /Wiki graph/ }));
    await userEvent.click(await screen.findByRole("option", { name: "board" }));
    expect(await screen.findByRole("button", { name: "From file" })).toBeTruthy();
  });

  it("exposes the 2D/3D mode and a layer switch per x-layer", async () => {
    globalThis.fetch = mockFetch() as unknown as typeof fetch;
    renderView();
    await screen.findByRole("button", { name: "Alpha" });
    expect(screen.getByRole("button", { name: /3D/ })).toBeTruthy();
    expect(screen.getByRole("switch", { name: "Level 0" })).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: /Wiki graph/ }));
    await userEvent.click(await screen.findByRole("option", { name: "board" }));
    expect(await screen.findByRole("switch", { name: "Deep" })).toBeTruthy();
  });

  it("shows an error state when the board API fails", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree")
        return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) });
      return Promise.resolve({
        ok: false,
        status: 500,
        json: () => Promise.resolve({ error: "boom" }),
      });
    }) as unknown as typeof fetch;
    renderView();
    await waitFor(() => expect(screen.getByText(/Board error: boom/)).toBeTruthy());
  });

  it("drills into a nested-canvas node and back with Escape", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree")
        return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) });
      if (url.startsWith("/api/wiki/board"))
        return Promise.resolve({ ok: true, json: () => Promise.resolve(NESTED_BOARD) });
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    const { container } = renderView();
    const node = await screen.findByRole("button", { name: "Investigation" });
    await userEvent.dblClick(node);
    await waitFor(() =>
      expect(container.querySelector(".board-breadcrumb")?.textContent).toContain("Investigation"),
    );
    // The active level is the child canvas; the root node is gone.
    expect(screen.queryByRole("button", { name: "Root" })).toBeNull();
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(container.querySelector(".board-breadcrumb")).toBeNull());
    expect(await screen.findByRole("button", { name: "Root" })).toBeTruthy();
  });

  it("navigates to an external .canvas target through the ?file= flow", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree")
        return Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) });
      if (
        url.includes("file=context%2Fwiki%2Fother.canvas") ||
        url.includes("file=context/wiki/other.canvas")
      )
        return Promise.resolve({ ok: true, json: () => Promise.resolve(EXTERNAL_BOARD) });
      if (url.startsWith("/api/wiki/board"))
        return Promise.resolve({ ok: true, json: () => Promise.resolve(EXTERNAL_REF_BOARD) });
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    renderView();
    const ref = await screen.findByRole("button", { name: "context/wiki/other.canvas" });
    await userEvent.dblClick(ref);
    expect(await screen.findByRole("button", { name: "External board" })).toBeTruthy();
  });
});

describe("renderNestedBody", () => {
  const node = {
    id: "nc",
    type: "nested-canvas",
    title: "Inv",
    x: 0,
    y: 0,
    width: 100,
    height: 100,
    canvas: { nodes: [{ id: "x", type: "text", x: 0, y: 0, width: 10, height: 10 }], edges: [] },
  } as CanvasNode;

  it("shows the opaque placeholder below the threshold", () => {
    const { container } = render(<>{renderNestedBody(node, 0.5)}</>);
    expect(container.querySelector(".nested-canvas--placeholder")).toBeTruthy();
  });

  it("renders a nested miniature above the threshold", () => {
    const { container } = render(<>{renderNestedBody(node, 1)}</>);
    expect(container.querySelector(".nested-canvas--mini")).toBeTruthy();
  });

  it("falls through (undefined) for a non-nested node", () => {
    expect(
      renderNestedBody({ id: "t", type: "text", x: 0, y: 0, width: 10, height: 10 }, 1),
    ).toBeUndefined();
  });
});
