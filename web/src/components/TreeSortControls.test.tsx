// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TreeSortControls } from "./TreeSortControls";
import { resetTreeSort, useTreeSort } from "../lib/treeSortStore";

function Probe() {
  const { key, dir } = useTreeSort();
  return <span data-testid="sort">{`${key}|${dir}`}</span>;
}

function renderControls() {
  render(
    <>
      <TreeSortControls />
      <Probe />
    </>,
  );
}

afterEach(() => {
  cleanup();
  resetTreeSort();
});

describe("TreeSortControls", () => {
  it("changes the shared sort key", async () => {
    renderControls();
    expect(screen.getByTestId("sort").textContent).toBe("created|desc");
    await userEvent.click(screen.getByRole("button", { name: /Sort entries by/ }));
    await userEvent.click(await screen.findByRole("option", { name: "Title" }));
    expect(screen.getByTestId("sort").textContent).toBe("title|desc");
  });

  it("toggles the direction", async () => {
    renderControls();
    await userEvent.click(screen.getByLabelText("Sort descending"));
    expect(screen.getByTestId("sort").textContent).toBe("created|asc");
  });
});
