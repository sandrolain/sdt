// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Breadcrumbs } from "./Breadcrumbs";

const writeText = vi.fn().mockResolvedValue(undefined);

beforeEach(() => {
  writeText.mockClear();
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    configurable: true,
  });
});

afterEach(cleanup);

function renderPath(path: string) {
  render(
    <MemoryRouter>
      <Breadcrumbs path={path} />
    </MemoryRouter>,
  );
}

describe("Breadcrumbs (corpus path bar)", () => {
  it("renders the full corpus path and a corpus-root link", () => {
    renderPath("context/wiki/backend/auth.md");
    expect(screen.getByText("context/wiki/backend/auth.md")).toBeTruthy();
    const home = screen.getByRole("link", { name: /Corpus root/ });
    expect(home.getAttribute("href")).toBe("/docs");
  });

  it("copies the path to the clipboard and flashes success", async () => {
    renderPath("context/plan/20260915-195559-viewer-fixes.md");
    const button = screen.getByRole("button", { name: /Copy path/ });
    fireEvent.click(button);
    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith("context/plan/20260915-195559-viewer-fixes.md"),
    );
    await waitFor(() => expect(screen.getByRole("button", { name: /Path copied/ })).toBeTruthy());
  });
});
