// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TreeSortControls } from "./TreeSortControls";
import { resetTreeSort, useTreeSort } from "../lib/treeSortStore";

function Probe() {
  const { key } = useTreeSort();
  return <span data-testid="sort">{key}</span>;
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
    expect(screen.getByTestId("sort").textContent).toBe("created_desc");
    await userEvent.click(screen.getByRole("button", { name: /Sort entries by/ }));
    await userEvent.click(await screen.findByRole("option", { name: "Title ASC" }));
    expect(screen.getByTestId("sort").textContent).toBe("title_asc");
  });

  it("selects a descending variant", async () => {
    renderControls();
    await userEvent.click(screen.getByRole("button", { name: /Sort entries by/ }));
    await userEvent.click(await screen.findByRole("option", { name: "Modified DESC" }));
    expect(screen.getByTestId("sort").textContent).toBe("modified_desc");
  });
});
