// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { BoardView } from "./BoardView";
import { normalizeBoard } from "../lib/canvas";

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

  it("exposes zoom controls", async () => {
    renderBoard();
    await userEvent.click(screen.getByRole("button", { name: "Zoom in" }));
    await userEvent.click(screen.getByRole("button", { name: "Zoom out" }));
    await userEvent.click(screen.getByRole("button", { name: "Fit" }));
    expect(screen.getByRole("group", { name: "Board zoom" })).toBeTruthy();
  });
});
