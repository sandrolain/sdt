// @vitest-environment jsdom
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

    await userEvent.click(item);
    await waitFor(() => {
      expect(screen.getByTestId("location").textContent).toBe("/docs/context/wiki/alpha.md");
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
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
});
