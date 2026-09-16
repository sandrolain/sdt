// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Link, MemoryRouter, Route, Routes } from "react-router-dom";
import { DocumentsPage } from "./DocumentsPage";
import { OpenDocsProvider } from "../components/OpenDocsProvider";
import { resetWikiIndexCache } from "../lib/wikiIndexLoader";
import { clearPreviewCache } from "../lib/preview";

const DOCS: Record<string, { path: string; frontmatter: string; markdown: string }> = {
  "context/a.md": {
    path: "context/a.md",
    frontmatter: "---\nkind: notes\ntitle: Alpha\n---\n",
    markdown: "# Alpha\n\nAlpha body text.",
  },
  "context/b.md": {
    path: "context/b.md",
    frontmatter: "---\nkind: notes\ntitle: Beta\n---\n",
    markdown: "# Beta\n\nBeta body text.",
  },
};

function mockFetch() {
  globalThis.fetch = vi.fn((url: string) => {
    if (url === "/api/tree") {
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ entries: [] }) });
    }
    const query = url.includes("?") ? url.slice(url.indexOf("?") + 1) : "";
    const path = decodeURIComponent(new URLSearchParams(query).get("path") ?? "");
    return Promise.resolve({ ok: true, json: () => Promise.resolve(DOCS[path] ?? {}) });
  }) as unknown as typeof fetch;
}

beforeEach(() => {
  resetWikiIndexCache();
  clearPreviewCache();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("DocumentsPage", () => {
  it("updates the document panel when the route changes", async () => {
    mockFetch();
    render(
      <MemoryRouter initialEntries={["/docs/context/a.md"]}>
        <OpenDocsProvider>
          <Link to="/docs/context/b.md">go b</Link>
          <Routes>
            <Route path="/docs/*" element={<DocumentsPage />} />
          </Routes>
        </OpenDocsProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByText(/Alpha body text/)).toBeTruthy();
    await userEvent.click(screen.getByText("go b"));
    expect(await screen.findByText(/Beta body text/)).toBeTruthy();
  });
});
