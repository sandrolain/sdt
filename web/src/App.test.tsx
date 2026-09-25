// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HashRouter } from "react-router-dom";
import { App } from "./App";

const MOCK_TREE = {
  entries: [
    { path: "context/wiki/alpha.md", kind: "wiki", title: "Alpha module", canvas: false },
    { path: "context/notes/note.md", kind: "notes", title: "A note", canvas: false },
    { path: "context/wiki/board.canvas", kind: "canvas", title: "board", canvas: true },
  ],
};

const MOCK_DOC = {
  path: "context/wiki/alpha.md",
  frontmatter: "---\nkind: wiki\ntitle: Alpha module\n---\n",
  markdown: "# Alpha module\n\nHello tokens.",
};

function mockFetch(url: string) {
  if (url === "/api/tree")
    return Promise.resolve({ ok: true, json: () => Promise.resolve(MOCK_TREE) });
  if (url.startsWith("/api/doc"))
    return Promise.resolve({ ok: true, json: () => Promise.resolve(MOCK_DOC) });
  return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
}

function renderApp() {
  return render(
    <HashRouter>
      <App />
    </HashRouter>,
  );
}

afterEach(() => {
  cleanup();
  window.location.hash = "";
});

describe("app shell", () => {
  it("renders tabs and the tree", async () => {
    globalThis.fetch = mockFetch as typeof fetch;
    renderApp();
    await screen.findByText("Alpha module");
    expect(screen.getByRole("link", { name: "Documents" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Wiki" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Search the corpus" })).toBeTruthy();
  });

  it("opens the search palette from the top bar", async () => {
    globalThis.fetch = mockFetch as typeof fetch;
    renderApp();
    await userEvent.click(screen.getByRole("button", { name: "Search the corpus" }));
    expect(await screen.findByLabelText("Search query")).toBeTruthy();
  });

  it("navigates from the tree to the doc detail", async () => {
    globalThis.fetch = mockFetch as typeof fetch;
    renderApp();
    const entry = await screen.findByTitle("context/wiki/alpha.md · Wiki");
    await userEvent.click(entry);
    await screen.findByText(/Hello tokens/);
  });

  it("tree kind filter shows canvas badge", async () => {
    globalThis.fetch = mockFetch as typeof fetch;
    renderApp();
    await screen.findByText("A note");
    const tree = screen.getByRole("complementary", { name: "Corpus tree" });
    // both the canvas folder and the wiki folder exist with human labels
    expect(within(tree).getAllByText("Canvas").length).toBeGreaterThan(0);
    expect(within(tree).getAllByText("Wiki").length).toBeGreaterThan(0);
  });

  it("cycles the theme preference on toggle", async () => {
    globalThis.fetch = mockFetch as typeof fetch;
    localStorage.setItem("sdt-theme", "light");
    renderApp();
    const toggle = screen.getByRole("button", { name: /theme/i }) as HTMLButtonElement;
    expect(document.documentElement.dataset.theme).toBe("light");
    await userEvent.click(toggle);
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(document.documentElement.dataset.themePref).toBe("dark");
  });
});
