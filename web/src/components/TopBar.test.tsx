// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ThemeProvider } from "./ThemeProvider";
import { TopBar } from "./TopBar";
import { currentReadingPrefs, resetReadingPrefs } from "../lib/readingPrefs";

function renderTopBar(initial: string) {
  render(
    <ThemeProvider>
      <MemoryRouter initialEntries={[initial]}>
        <TopBar onOpenSearch={vi.fn()} />
      </MemoryRouter>
    </ThemeProvider>,
  );
}

afterEach(() => {
  cleanup();
  resetReadingPrefs();
  localStorage.clear();
});

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

  it("highlights the Graph entry under the wiki graph route", () => {
    renderTopBar("/wiki/graph");
    expect(screen.getByRole("link", { name: /Graph/ }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("link", { name: /Documents/ }).getAttribute("aria-current")).toBeNull();
  });

  it("opens the reading settings from the top bar and applies them", async () => {
    renderTopBar("/docs");
    await userEvent.click(screen.getByRole("button", { name: "Reading settings" }));
    const largest = await screen.findByRole("button", { name: "Text size: Largest" });
    expect(
      screen.getByRole("button", { name: "Text size: Default" }).getAttribute("aria-pressed"),
    ).toBe("true");

    await userEvent.click(largest);
    await waitFor(() => expect(currentReadingPrefs().fontScale).toBe(1.3));
    expect(document.documentElement.style.getPropertyValue("--reading-scale")).toBe("1.3");

    const measure = screen.getByRole("switch", { name: /Limit line length/ });
    await userEvent.click(measure);
    await waitFor(() => expect(currentReadingPrefs().measureOn).toBe(false));
    expect(document.documentElement.style.getPropertyValue("--reading-measure")).toBe("100%");
  });

  it("keeps the settings dialog labelled and keyboard reachable", async () => {
    renderTopBar("/docs");
    const trigger = screen.getByRole("button", { name: "Reading settings" });
    trigger.focus();
    expect(document.activeElement).toBe(trigger);
    await userEvent.keyboard("{Enter}");
    expect(await screen.findByRole("dialog", { name: "Reading settings" })).toBeTruthy();
    expect(screen.getByRole("group", { name: "Text size" })).toBeTruthy();
  });
});
