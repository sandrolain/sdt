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

  // The reported defect: on a view's route the Documents item stayed marked next to
  // the current one, because the match was a prefix plus a hand-written exception for
  // the map. The rule now reads the bar's own route table, so exactly one item is
  // marked everywhere — including the document-detail path beneath `/docs`, which
  // Documents still owns.
  it("marks exactly one item for every route the bar owns", () => {
    const cases: { path: string; active: string }[] = [
      { path: "/docs", active: "/docs" },
      { path: "/docs/context/analysis/foo.md", active: "/docs" },
      { path: "/docs/dashboard", active: "/docs/dashboard" },
      { path: "/docs/map", active: "/docs/map" },
      { path: "/docs/gallery", active: "/docs/gallery" },
      { path: "/docs/timeline", active: "/docs/timeline" },
      { path: "/docs/kanban", active: "/docs/kanban" },
      { path: "/wiki/graph", active: "/wiki/graph" },
      { path: "/wiki/board", active: "/wiki/board" },
      // The board keeps the drill path in its link, so it stays the marked item.
      { path: "/wiki/board/context/notes/a.md", active: "/wiki/board/context/notes/a.md" },
    ];
    for (const { path, active } of cases) {
      cleanup();
      renderTopBar(path);
      const marked = screen
        .getAllByRole("link")
        .filter((link) => link.className.includes("is-active"))
        .map((link) => link.getAttribute("href"));
      expect([path, marked]).toEqual([path, [active]]);
    }
  });

  it("highlights Documents at the section root", () => {
    renderTopBar("/docs");
    expect(screen.getByRole("link", { name: /Documents/ }).getAttribute("aria-current")).toBe(
      "page",
    );
  });

  it("orders the tabs with Dashboard first and one hairline-separated group per section", () => {
    renderTopBar("/docs");
    const nav = screen.getByRole("navigation", { name: "Primary" });
    const labels = Array.from(nav.querySelectorAll("a.top-bar__tab")).map((a) => {
      // strip the leading Material Symbols ligature (the icon's text content)
      const glyph = a.querySelector(".ms-icon")?.textContent ?? "";
      return (a.textContent ?? "").replace(glyph, "").trim();
    });
    expect(labels).toEqual([
      "Dashboard",
      "Documents",
      "Map",
      "Gallery",
      "Timeline",
      "Kanban",
      "Graph",
      "Board",
    ]);
    // three sibling groups → the existing `.top-bar__group + .top-bar__group`
    // rule draws the hairline separator; no new CSS is added
    expect(nav.querySelectorAll(":scope > .top-bar__group").length).toBe(3);
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
