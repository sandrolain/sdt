// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { DashboardView } from "./DashboardView";
import { GalleryView } from "./GalleryView";
import { KanbanView } from "./KanbanView";
import { TimelineView } from "./TimelineView";
import { resetCorpusIndexCache } from "../lib/corpusIndex";

const MOCK_TREE = {
  entries: [
    {
      path: "context/notes/a.md",
      kind: "notes",
      title: "Alpha",
      status: "active",
      created: "2026-09-15",
      modified: "2026-09-16",
      categories: ["research"],
      summary: "s",
    },
    {
      path: "context/analysis/b.md",
      kind: "analysis",
      title: "Beta",
      status: "draft",
      created: "2026-09-14",
      categories: ["improvement"],
    },
  ],
};

const MOCK_TODO = {
  items: [
    { id: "t-done", text: "Shipped idea", created: "2026-09-10", done: true },
    { id: "t-open", text: "Open idea", created: "2026-09-12", done: false },
  ],
};

function mockFetch() {
  globalThis.fetch = vi.fn((url: string) => {
    if (url === "/api/tree") {
      return Promise.resolve({ ok: true, json: () => Promise.resolve(MOCK_TREE) });
    }
    if (url === "/api/todo") {
      return Promise.resolve({ ok: true, json: () => Promise.resolve(MOCK_TODO) });
    }
    return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
  }) as unknown as typeof fetch;
}

function renderView(node: React.ReactNode) {
  return render(<MemoryRouter>{node}</MemoryRouter>);
}

afterEach(() => {
  cleanup();
  resetCorpusIndexCache();
  vi.restoreAllMocks();
});

describe("corpus views", () => {
  it("gallery renders a card per document", async () => {
    mockFetch();
    renderView(<GalleryView />);
    expect(await screen.findByText("Alpha")).toBeTruthy();
    expect(screen.getByText("Beta")).toBeTruthy();
  });

  it("timeline renders the documents grouped by day", async () => {
    mockFetch();
    renderView(<TimelineView />);
    expect(await screen.findByText("Alpha")).toBeTruthy();
    expect(screen.getByText("Beta")).toBeTruthy();
  });

  it("kanban renders status columns", async () => {
    mockFetch();
    renderView(<KanbanView />);
    expect(await screen.findByText("Alpha")).toBeTruthy();
    expect(screen.getByText("active")).toBeTruthy();
  });

  it("dashboard renders the document total", async () => {
    mockFetch();
    renderView(<DashboardView />);
    expect(await screen.findByText("2")).toBeTruthy();
    expect(screen.getByText("Recent activity")).toBeTruthy();
  });

  it("dashboard surfaces the TODO inbox, undone first", async () => {
    mockFetch();
    renderView(<DashboardView />);
    const open = await screen.findByText("Open idea");
    expect(screen.getByText("Shipped idea")).toBeTruthy();
    const texts = Array.from(
      document.querySelectorAll(".dashboard__todos .dashboard__todo-text"),
    ).map((n) => n.textContent);
    expect(texts).toEqual(["Open idea", "Shipped idea"]);
    expect(open.closest(".dashboard__todo")?.classList.contains("is-done")).toBe(false);
  });

  it("dashboard shows an empty TODO inbox without failing", async () => {
    globalThis.fetch = vi.fn((url: string) => {
      if (url === "/api/tree") {
        return Promise.resolve({ ok: true, json: () => Promise.resolve(MOCK_TREE) });
      }
      if (url === "/api/todo") {
        return Promise.resolve({ ok: true, json: () => Promise.resolve({ items: [] }) });
      }
      return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
    }) as unknown as typeof fetch;
    renderView(<DashboardView />);
    expect(await screen.findByText("No ideas in the inbox.")).toBeTruthy();
  });
});
