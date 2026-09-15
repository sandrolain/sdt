// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useParams } from "react-router-dom";
import { OpenDocsRail } from "./OpenDocsRail";

const DOCS: Record<string, { path: string; frontmatter: string; markdown: string }> = {
  "context/a.md": {
    path: "context/a.md",
    frontmatter: "---\nkind: notes\ntitle: Alpha\n---\n",
    markdown: "# Alpha\n\nSee [Go B](b.md) for more.",
  },
  "context/b.md": {
    path: "context/b.md",
    frontmatter: "---\nkind: notes\ntitle: Beta\n---\n",
    markdown: "# Beta\n\nSecond document.",
  },
};

function Harness() {
  const path = useParams()["*"];
  return <OpenDocsRail path={path} />;
}

function renderRail() {
  render(
    <MemoryRouter initialEntries={["/docs/context/a.md"]}>
      <Routes>
        <Route path="/docs/*" element={<Harness />} />
      </Routes>
    </MemoryRouter>,
  );
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function mockDoc() {
  globalThis.fetch = vi.fn((url: string) => {
    const query = url.includes("?") ? url.slice(url.indexOf("?") + 1) : "";
    const path = decodeURIComponent(new URLSearchParams(query).get("path") ?? "");
    return Promise.resolve({ ok: true, json: () => Promise.resolve(DOCS[path]) });
  }) as unknown as typeof fetch;
}

describe("OpenDocsRail", () => {
  it("opens a body link as a new tab on the right and marks it active", async () => {
    mockDoc();
    renderRail();
    expect(await screen.findByRole("tab", { name: "A" })).toBeTruthy();

    await userEvent.click(await screen.findByText("Go B"));
    expect(await screen.findByRole("tab", { name: "B" })).toBeTruthy();
    expect(screen.getByRole("tab", { name: "B", selected: true })).toBeTruthy();
    expect(await screen.findByText(/Second document/)).toBeTruthy();
  });

  it("lazily mounts a slide only after it has been active", async () => {
    mockDoc();
    renderRail();
    await screen.findByRole("tab", { name: "A" });
    // Beta is not open yet, so no Beta slide exists
    expect(screen.queryByText(/Second document/)).toBeNull();
  });

  it("closes a tab and falls back to the previous one", async () => {
    mockDoc();
    renderRail();
    await screen.findByRole("tab", { name: "A" });
    await userEvent.click(await screen.findByText("Go B"));
    await screen.findByRole("tab", { name: "B" });

    await userEvent.click(screen.getByLabelText("Close B"));
    expect(screen.queryByRole("tab", { name: "B" })).toBeNull();
    expect(screen.getByRole("tab", { name: "A", selected: true })).toBeTruthy();
  });
});
