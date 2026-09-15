// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { RelatedPanel } from "./RelatedPanel";

const REL = {
  id: "a",
  title: "Alpha",
  outbound: {
    depends_on: [{ target: "b", title: "Beta", kind: "relation", path: "context/wiki/b.md" }],
  },
  inbound: {
    part_of: [{ source: "c", title: "Gamma", kind: "relation", path: "context/wiki/c.md" }],
  },
};

function mockFetch(payload: unknown, ok = true) {
  return vi.fn(() => Promise.resolve({ ok, json: () => Promise.resolve(payload) }));
}

function renderPanel() {
  render(
    <MemoryRouter>
      <RelatedPanel id="a" />
    </MemoryRouter>,
  );
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("RelatedPanel", () => {
  it("groups relations by verb with direction arrows and links", async () => {
    globalThis.fetch = mockFetch(REL) as unknown as typeof fetch;
    renderPanel();
    expect(await screen.findByText("Beta")).toBeTruthy();
    expect(screen.getByText("Gamma")).toBeTruthy();
    expect(screen.getByText("depends_on")).toBeTruthy();
    expect(screen.getByText("part_of")).toBeTruthy();
    expect(screen.getByLabelText("outbound")).toBeTruthy();
    expect(screen.getByLabelText("inbound")).toBeTruthy();
    expect(screen.getByText("1 out · 1 in")).toBeTruthy();
  });

  it("shows an empty state", async () => {
    globalThis.fetch = mockFetch({ id: "a", title: "A" }) as unknown as typeof fetch;
    renderPanel();
    expect(await screen.findByText("No relations.")).toBeTruthy();
  });

  it("shows an error state", async () => {
    globalThis.fetch = mockFetch({ error: "boom" }, false) as unknown as typeof fetch;
    renderPanel();
    expect(await screen.findByText(/relations error: boom/)).toBeTruthy();
  });
});
