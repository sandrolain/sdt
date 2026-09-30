// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ReferencedBy } from "./ReferencedBy";

function mockBacklinks(payload: unknown, ok = true) {
  globalThis.fetch = vi.fn((url: string) => {
    if (url.startsWith("/api/backlinks")) {
      return Promise.resolve({ ok, json: () => Promise.resolve(payload) });
    }
    return Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve(null) });
  }) as unknown as typeof fetch;
}

function renderPanel(props: Partial<Parameters<typeof ReferencedBy>[0]> = {}) {
  return render(
    <MemoryRouter>
      <ReferencedBy path="context/analysis/analy-x.md" {...props} />
    </MemoryRouter>,
  );
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("ReferencedBy", () => {
  it("lists the referring documents with the reason for each edge", async () => {
    mockBacklinks({
      path: "context/analysis/analy-x.md",
      total: 2,
      referrers: [
        { path: "context/plan/plan-x.md", title: "Plan X", kind: "plan", via: "sources" },
        { path: "context/tasks/t.md", title: "Task", kind: "tasks", via: "relation" },
      ],
    });
    renderPanel();
    const plan = await screen.findByRole("link", { name: /Plan X/ });
    expect(plan.getAttribute("href")).toBe("/docs/context/plan/plan-x.md");
    // no heading when the host renders its own card title
    expect(screen.queryByRole("heading")).toBeNull();
    expect(screen.getByText("source")).toBeTruthy();
    expect(screen.getByText("derives from")).toBeTruthy();
  });

  it("derives a readable title when a referrer has none", async () => {
    mockBacklinks({
      path: "x",
      total: 1,
      referrers: [{ path: "context/notes/20260901-note.md", via: "links" }],
    });
    renderPanel();
    const link = await screen.findByRole("link");
    expect(link.getAttribute("href")).toBe("/docs/context/notes/20260901-note.md");
    // the filename cascade turns the path into "Note", never a raw path
    expect(link.textContent).not.toContain("context/notes/20260901-note.md");
  });

  it("says so when nothing references the document", async () => {
    mockBacklinks({ path: "x", total: 0, referrers: [] });
    renderPanel();
    expect(await screen.findByText(/No document references/)).toBeTruthy();
  });

  it("reports a failed request without hiding the card", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({ error: "boom" }) }),
    ) as unknown as typeof fetch;
    renderPanel();
    expect(await screen.findByText(/backlinks error/)).toBeTruthy();
  });

  it("requests the path it is given", async () => {
    mockBacklinks({ path: "x", total: 0, referrers: [] });
    renderPanel({ path: "context/plan/plan x.md" });
    await screen.findByText(/No document references/);
    expect((globalThis.fetch as unknown as { mock: { calls: string[][] } }).mock.calls[0][0]).toBe(
      "/api/backlinks?path=context%2Fplan%2Fplan%20x.md",
    );
  });

  it("renders the heading with the count when the host has no title of its own", async () => {
    mockBacklinks({
      path: "x",
      total: 2,
      referrers: [
        { path: "context/plan/plan-x.md", title: "Plan X", via: "sources" },
        { path: "context/plan/plan-y.md", title: "Plan Y", via: "links" },
      ],
    });
    renderPanel({ heading: "Referenced by", compact: true });
    const heading = await screen.findByRole("heading", { name: /Referenced by/ });
    expect(heading.textContent).toContain("2");
  });

  it("hides the reason in the compact list", async () => {
    mockBacklinks({
      path: "x",
      total: 1,
      referrers: [{ path: "context/plan/plan-x.md", title: "Plan X", via: "sources" }],
    });
    renderPanel({ compact: true });
    await screen.findByRole("link", { name: /Plan X/ });
    expect(screen.queryByText("source")).toBeNull();
  });
});
