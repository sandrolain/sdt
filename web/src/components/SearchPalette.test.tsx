// @vitest-environment jsdom
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { SearchPalette } from "./SearchPalette";

function LocationProbe() {
  const loc = useLocation();
  return <span data-testid="location">{loc.pathname}</span>;
}

function renderPalette(onOpenChange = vi.fn()) {
  const view = render(
    <MemoryRouter initialEntries={["/docs"]}>
      <Routes>
        <Route path="/docs/*" element={<LocationProbe />} />
      </Routes>
      <SearchPalette open onOpenChange={onOpenChange} />
    </MemoryRouter>,
  );
  return { container: view.container, onOpenChange };
}

/** Palette whose open state the test drives, so a close/reopen cycle is real. */
function renderToggleablePalette() {
  function Harness() {
    const [open, setOpen] = useState(true);
    return (
      <MemoryRouter initialEntries={["/docs"]}>
        <Routes>
          <Route path="/docs/*" element={<LocationProbe />} />
        </Routes>
        <button type="button" onClick={() => setOpen((value) => !value)}>
          toggle
        </button>
        <SearchPalette open={open} onOpenChange={setOpen} />
      </MemoryRouter>
    );
  }
  return render(<Harness />);
}

function mockSearch(payload: unknown, ok = true) {
  return vi.fn((url: string) => {
    if (url.startsWith("/api/search")) {
      return Promise.resolve({ ok, json: () => Promise.resolve(payload) });
    }
    return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
  });
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("SearchPalette", () => {
  it("prompts for a longer query before searching", () => {
    globalThis.fetch = mockSearch({ results: [], total: 0 }) as unknown as typeof fetch;
    renderPalette();
    expect(screen.getByText(/Type at least 2 characters/)).toBeTruthy();
  });

  it("renders ranked results and navigates on select", async () => {
    globalThis.fetch = mockSearch({
      results: [
        {
          path: "context/wiki/alpha.md",
          kind: "wiki",
          title: "Alpha module",
          created: "2026-09-10",
          score: 2,
          snippet: "alpha handles tokens",
        },
      ],
      total: 1,
    }) as unknown as typeof fetch;
    const { onOpenChange } = renderPalette();

    await userEvent.type(screen.getByLabelText("Search query"), "tokens");
    const item = await screen.findByText("Alpha module");
    expect(screen.getByText("1 result")).toBeTruthy();
    // the ISO value is formatted for display, not shown raw
    const meta = document.querySelector(".search-result__meta")?.textContent ?? "";
    expect(meta).not.toContain("2026-09-10");
    expect(meta).toMatch(/2026/);

    await userEvent.click(item);
    await waitFor(() => {
      expect(screen.getByTestId("location").textContent).toBe("/docs/context/wiki/alpha.md");
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("renders the modified date when present and different from created", async () => {
    globalThis.fetch = mockSearch({
      results: [
        {
          path: "context/wiki/beta.md",
          kind: "wiki",
          title: "Beta module",
          created: "2026-09-11",
          modified: "2026-09-20",
          score: 2,
          snippet: "beta handles tokens",
        },
      ],
      total: 1,
    }) as unknown as typeof fetch;
    renderPalette();
    await userEvent.type(screen.getByLabelText("Search query"), "tokens");
    await screen.findByText("Beta module");
    const meta = document.querySelector(".search-result__meta")?.textContent ?? "";
    expect(meta).toContain("updated");
    expect(meta).not.toContain("2026-09-20T");
  });

  it("shows a no-results state", async () => {
    globalThis.fetch = mockSearch({ results: [], total: 0 }) as unknown as typeof fetch;
    renderPalette();
    await userEvent.type(screen.getByLabelText("Search query"), "zzz");
    expect(await screen.findByText("No results.")).toBeTruthy();
  });

  it("shows an error state when the search fails", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({ error: "boom" }) }),
    ) as unknown as typeof fetch;
    renderPalette();
    await userEvent.type(screen.getByLabelText("Search query"), "tokens");
    expect(await screen.findByText(/Search error: boom/)).toBeTruthy();
  });

  it("composes the kind filter into the request", async () => {
    const fetchMock = mockSearch({ results: [], total: 0 });
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    renderPalette();

    await userEvent.selectOptions(screen.getByLabelText("Kind"), "wiki");
    await userEvent.type(screen.getByLabelText("Search query"), "tokens");
    await waitFor(() => {
      const urls = fetchMock.mock.calls.map((c) => c[0]);
      expect(urls.some((u) => u.includes("kind=wiki"))).toBe(true);
    });
  });

  it("renders many results inside the scrollable list host", async () => {
    const results = Array.from({ length: 25 }, (_, i) => ({
      path: `context/wiki/doc${i}.md`,
      kind: "wiki",
      title: `Doc ${i}`,
      created: "2026-09-10",
      score: 1,
      snippet: `body ${i}`,
    }));
    globalThis.fetch = mockSearch({ results, total: results.length }) as unknown as typeof fetch;
    renderPalette();
    await userEvent.type(screen.getByLabelText("Search query"), "doc");
    expect(await screen.findByText("25 results")).toBeTruthy();
    const host = document.querySelector(".search-palette__list");
    expect(host).not.toBeNull();
    expect(host!.querySelectorAll("[cmdk-item]").length).toBe(results.length);
  });

  it("exposes the filter row as a labelled group and arrows from it into the results", async () => {
    globalThis.fetch = mockSearch({
      results: [
        { path: "context/wiki/alpha.md", kind: "wiki", title: "Alpha", score: 1, snippet: "a" },
        { path: "context/wiki/beta.md", kind: "wiki", title: "Beta", score: 1, snippet: "b" },
      ],
      total: 2,
    }) as unknown as typeof fetch;
    renderPalette();
    await userEvent.type(screen.getByLabelText("Search query"), "alpha");
    expect(await screen.findByText("Alpha")).toBeTruthy();

    const row = screen.getByRole("group", { name: "Search filters" });
    row.focus();
    expect(document.activeElement).toBe(row);
    // focus parked on the row must not dead-end: cmdk keeps the roving selection
    await userEvent.keyboard("{ArrowDown}");
    expect(document.querySelectorAll('[cmdk-item][aria-selected="true"]')).toHaveLength(1);
  });

  it("leaves arrow keys to the control that received them", async () => {
    globalThis.fetch = mockSearch({ results: [], total: 0 }) as unknown as typeof fetch;
    renderPalette();
    const objective = screen.getByLabelText("Objective filter");
    objective.focus();
    await userEvent.type(objective, "viewer");
    expect((objective as HTMLInputElement).value).toBe("viewer");
  });

  it("resets every filter on close, so a reopen cannot show a stale empty result", async () => {
    const fetchMock = mockSearch({ results: [], total: 0 });
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    renderToggleablePalette();

    await userEvent.type(screen.getByLabelText("Search query"), "tokens");
    await userEvent.selectOptions(screen.getByLabelText("Kind"), "wiki");
    await waitFor(() => {
      const urls = fetchMock.mock.calls.map((c) => String(c[0]));
      expect(urls.some((u) => u.includes("kind=wiki"))).toBe(true);
    });

    // Escape closes the palette; the toggle is outside it and reachable again
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await userEvent.click(screen.getByRole("button", { name: "toggle" }));

    expect((screen.getByLabelText("Kind") as HTMLSelectElement).value).toBe("");
    expect((screen.getByLabelText("Objective filter") as HTMLInputElement).value).toBe("");
    expect((screen.getByLabelText("Search query") as HTMLInputElement).value).toBe("tokens");
  });
});
