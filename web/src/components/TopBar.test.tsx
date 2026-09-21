// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ThemeProvider } from "./ThemeProvider";
import { TopBar } from "./TopBar";

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
  it("renders the SDT brand mark", () => {
    renderTopBar("/docs");
    expect(screen.getByRole("img", { name: "SDT" }).getAttribute("src")).toBe("/sdt-logo.svg");
  });

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
