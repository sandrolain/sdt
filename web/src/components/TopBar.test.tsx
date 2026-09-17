// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { TopBar } from "./TopBar";
import { ThemeProvider } from "./ThemeProvider";

function renderTopBar(initial: string) {
  render(
    <ThemeProvider>
      <MemoryRouter initialEntries={[initial]}>
        <TopBar onOpenSearch={vi.fn()} />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

afterEach(cleanup);

describe("TopBar", () => {
  it("highlights Documents on a full document path", () => {
    renderTopBar("/docs/context/analysis/foo.md");
    expect(screen.getByRole("link", { name: /Documents/ }).getAttribute("aria-current")).toBe(
      "page",
    );
  });

  it("highlights Documents at the section root", () => {
    renderTopBar("/docs");
    expect(screen.getByRole("link", { name: /Documents/ }).getAttribute("aria-current")).toBe(
      "page",
    );
  });

  it("highlights Wiki under the wiki route", () => {
    renderTopBar("/wiki/graph");
    expect(screen.getByRole("link", { name: /Wiki/ }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("link", { name: /Documents/ }).getAttribute("aria-current")).toBeNull();
  });
});
