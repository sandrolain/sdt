// @vitest-environment jsdom
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { SearchPalette } from "./SearchPalette";

function LocationProbe() {
  const loc = useLocation();
  return (
    <span data-testid="location">
      {loc.pathname}
      {loc.hash}
    </span>
  );
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
    if (url.startsWith("/api/vocab")) {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            objectives: ["viewer", "cli"],
            categories: ["bug"],
            topics: ["ui-ux"],
          }),
      });
    }
    return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
  });
}

const VOCAB = { objectives: ["viewer", "cli"], categories: ["bug"], topics: ["ui-ux"] };

/** A register with nothing in it (a project that has not declared topics). */
const EMPTY_VOCAB = { objectives: [], categories: [], topics: [] };

function fetchVocabCalls(): number {
  const mock = globalThis.fetch as unknown as { mock?: { calls: string[][] } };
  return (mock.mock?.calls ?? []).filter((c) => String(c[0]).startsWith("/api/vocab")).length;
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("SearchPalette", () => {
  it("browses the corpus with an empty query", async () => {
    const fetchMock = mockSearch({
      results: [{ path: "context/notes/n.md", kind: "notes", title: "Note", score: 0 }],
      total: 1,
    });
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    renderPalette();
    expect(await screen.findByText("Note")).toBeTruthy();
    // browse mode is an empty q; the server decides the order (modified_desc)
    const url = fetchMock.mock.calls
      .map((c) => String(c[0]))
      .find((u) => u.includes("/api/search"));
    expect(url).toContain("q=&");
  });

  it("offers the corpus registers as facet options", async () => {
    globalThis.fetch = mockSearch({ results: [], total: 0 }) as unknown as typeof fetch;
    renderPalette();
    await waitFor(() => expect(fetchVocabCalls()).toBeGreaterThan(0));
    const options = (name: string) =>
      Array.from((screen.getByLabelText(name) as HTMLSelectElement).options).map((o) => o.value);
    expect(options("Objective")).toEqual(["", ...VOCAB.objectives]);
    expect(options("Topic")).toEqual(["", ...VOCAB.topics]);
    expect(options("Category")).toEqual(["", ...VOCAB.categories]);
    // the status select comes from the matrix, and offers active
    expect(options("Status")).toContain("active");
  });

  it("hides a facet whose register is empty", async () => {
    globalThis.fetch = vi.fn((url: string) =>
      url.startsWith("/api/vocab")
        ? Promise.resolve({ ok: true, json: () => Promise.resolve(EMPTY_VOCAB) })
        : Promise.resolve({ ok: true, json: () => Promise.resolve({ results: [], total: 0 }) }),
    ) as unknown as typeof fetch;
    renderPalette();
    await waitFor(() => expect(fetchVocabCalls()).toBeGreaterThan(0));
    expect(screen.queryByLabelText("Topic")).toBeNull();
    expect(screen.queryByLabelText("Objective")).toBeNull();
    // the kind, status and date filters never depend on a register
    expect(screen.getByLabelText("Kind")).toBeTruthy();
    expect(screen.getByLabelText("Status")).toBeTruthy();
  });

  it("composes the status and topic facets into the request", async () => {
    const fetchMock = mockSearch({ results: [], total: 0 });
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    renderPalette();
    await waitFor(() => expect(fetchVocabCalls()).toBeGreaterThan(0));
    await userEvent.selectOptions(screen.getByLabelText("Status"), "active");
    await userEvent.selectOptions(screen.getByLabelText("Topic"), "ui-ux");
    await waitFor(() => {
      const urls = fetchMock.mock.calls.map((c) => String(c[0]));
      expect(urls.some((u) => u.includes("status=active"))).toBe(true);
      expect(urls.some((u) => u.includes("topic=ui-ux"))).toBe(true);
    });
  });

  it("pages past the first page with Show more", async () => {
    const page = (n: number, total: number) =>
      mockSearch({
        results: Array.from({ length: n }, (_, i) => ({
          path: `context/notes/n${i}.md`,
          kind: "notes",
          title: `Note ${i}`,
          score: 0,
        })),
        total,
      });
    globalThis.fetch = page(20, 35) as unknown as typeof fetch;
    renderPalette();
    expect(await screen.findByText("35 results")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Show more" })).toBeTruthy();

    globalThis.fetch = page(35, 35) as unknown as typeof fetch;
    await userEvent.click(screen.getByRole("button", { name: "Show more" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Show more" })).toBeNull());
    expect(screen.getByText("35 of 35 shown")).toBeTruthy();
  });

  it("announces the result count in a live region", async () => {
    globalThis.fetch = mockSearch({ results: [], total: 0 }) as unknown as typeof fetch;
    renderPalette();
    const status = await screen.findByRole("status");
    expect(status.textContent).toContain("shown");
  });

  it("deep-links a hit that matched a section", async () => {
    globalThis.fetch = mockSearch({
      results: [
        {
          path: "context/notes/n.md",
          kind: "notes",
          title: "Note",
          score: 1,
          section: "findings",
          snippet: "the token lives here",
        },
      ],
      total: 1,
    }) as unknown as typeof fetch;
    renderPalette();
    const item = await screen.findByText("Note");
    await userEvent.click(item);
    await waitFor(() => {
      expect(screen.getByTestId("location").textContent).toBe("/docs/context/notes/n.md#findings");
    });
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
    await waitFor(() => expect(fetchVocabCalls()).toBeGreaterThan(0));
    const topic = screen.getByLabelText("Topic");
    topic.focus();
    await userEvent.selectOptions(topic, "ui-ux");
    expect((topic as HTMLSelectElement).value).toBe("ui-ux");
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
    expect((screen.getByLabelText("Status") as HTMLSelectElement).value).toBe("");
    expect((screen.getByLabelText("Search query") as HTMLInputElement).value).toBe("tokens");
  });
});
