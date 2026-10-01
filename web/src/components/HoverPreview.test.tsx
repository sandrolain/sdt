// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { HoverPreview } from "./HoverPreview";
import { clearPreviewCache } from "../lib/preview";

const DOC = {
  frontmatter:
    "---\ntitle: Preview Title\nsummary: A short summary\ncreated: 2026-09-01\nupdated: 2026-09-10\n---\n",
  markdown: "# Preview Title\n\nBody with <script>alert(1)</script>",
};

afterEach(() => {
  cleanup();
  clearPreviewCache();
  vi.restoreAllMocks();
});

function mockDoc() {
  globalThis.fetch = vi.fn(() =>
    Promise.resolve({ ok: true, json: () => Promise.resolve(DOC) }),
  ) as unknown as typeof fetch;
}

function renderLink() {
  render(
    <div className="doc-rendered">
      <a href="/docs/context/b.md">Go B</a>
    </div>,
  );
  render(<HoverPreview delay={0} leaveDelay={0} />);
}

describe("HoverPreview", () => {
  it("shows a metadata card with title, summary, dates and path", async () => {
    mockDoc();
    renderLink();
    fireEvent.mouseOver(screen.getByText("Go B"));
    const tip = await screen.findByRole("tooltip");
    expect(tip.textContent).toContain("Preview Title");
    expect(tip.textContent).toContain("A short summary");
    expect(tip.textContent).toContain("2026");
    expect(tip.textContent).toContain("context/b.md");
    expect(tip.parentElement).toBe(document.body);
  });

  it("shows the frontmatter image thumbnail", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            frontmatter: "---\ntitle: With image\nimage: context/assets/cover.png\n---\n",
            markdown: "",
          }),
      }),
    ) as unknown as typeof fetch;
    renderLink();
    fireEvent.mouseOver(screen.getByText("Go B"));
    const tip = await screen.findByRole("tooltip");
    const img = tip.querySelector("img.hover-preview__image");
    expect(img?.getAttribute("src")).toBe("/api/file?path=context%2Fassets%2Fcover.png");
  });

  it("stays open while the pointer is over the card", async () => {
    mockDoc();
    renderLink();
    fireEvent.mouseOver(screen.getByText("Go B"));
    const tip = await screen.findByRole("tooltip");
    fireEvent.mouseOut(screen.getByText("Go B"));
    fireEvent.mouseEnter(tip);
    await new Promise((r) => setTimeout(r, 10));
    expect(screen.queryByRole("tooltip")).toBeTruthy();
  });

  it("hides shortly after the pointer leaves the link", async () => {
    mockDoc();
    renderLink();
    fireEvent.mouseOver(screen.getByText("Go B"));
    await screen.findByRole("tooltip");
    fireEvent.mouseOut(screen.getByText("Go B"));
    await waitFor(() => expect(screen.queryByRole("tooltip")).toBeNull());
  });
  it("opens on keyboard focus, not only on hover", async () => {
    mockDoc();
    renderLink();
    fireEvent.focus(screen.getByText("Go B"));
    const tip = await screen.findByRole("tooltip");
    expect(tip.textContent).toContain("Preview Title");
  });

  it("associates the card with the link through aria-describedby", async () => {
    mockDoc();
    renderLink();
    const link = screen.getByText("Go B");
    fireEvent.focus(link);
    const tip = await screen.findByRole("tooltip");
    await waitFor(() => expect(link.getAttribute("aria-describedby")).toBe(tip.id));
    expect(tip.id).toBeTruthy();

    // leaving the link drops the association again
    fireEvent.focusOut(link);
    await waitFor(() => expect(link.getAttribute("aria-describedby")).toBeNull());
  });
});
