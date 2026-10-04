// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { Breadcrumbs } from "./Breadcrumbs";
import { resetCorpusIndexCache } from "../lib/corpusIndex";

const TREE = {
  entries: [
    { path: "context/tasks/t1.md", kind: "tasks", title: "First", plan: "context/plan/p1.md" },
    { path: "context/tasks/t2.md", kind: "tasks", title: "Second" },
    { path: "context/plan/p1.md", kind: "plan", title: "Plan one" },
  ],
};

function LocationProbe() {
  const loc = useLocation();
  return <span data-testid="probe">{loc.pathname}</span>;
}

function mockTree() {
  globalThis.fetch = vi.fn((url: string) =>
    url === "/api/tree"
      ? Promise.resolve({ ok: true, json: () => Promise.resolve(TREE) })
      : Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) }),
  ) as unknown as typeof fetch;
}

const writeText = vi.fn().mockResolvedValue(undefined);

beforeEach(() => {
  writeText.mockClear();
  resetCorpusIndexCache();
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    configurable: true,
  });
});

afterEach(cleanup);

function renderPath(path: string) {
  render(
    <MemoryRouter initialEntries={["/docs"]}>
      <Routes>
        <Route path="/docs/*" element={<LocationProbe />} />
      </Routes>
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

describe("Breadcrumbs relations", () => {
  it("links the plan a task file belongs to", async () => {
    mockTree();
    renderPath("context/tasks/t1.md");
    const link = await screen.findByRole("link", { name: /Plan one/ });
    expect(link.getAttribute("href")).toBe("/docs/context/plan/p1.md");
  });

  it("offers no relation for a document with no typed parent", async () => {
    mockTree();
    renderPath("context/plan/p1.md");
    // the index resolves (no error state), but this plan declares no analysis
    await waitFor(() => expect(screen.getByText("context/plan/p1.md")).toBeTruthy());
    expect(screen.queryByRole("link", { name: /^Plan/ })).toBeNull();
    expect(screen.queryByRole("link", { name: /^Analysis/ })).toBeNull();
  });

  it("keeps the path and the copy button when the index is unavailable", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({ error: "boom" }) }),
    ) as unknown as typeof fetch;
    renderPath("context/tasks/t1.md");
    expect(screen.getByText("context/tasks/t1.md")).toBeTruthy();
    expect(screen.getByRole("button", { name: /Copy path/ })).toBeTruthy();
  });
});
