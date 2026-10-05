// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { WikiBoardView } from "./WikiBoardView";
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
});
