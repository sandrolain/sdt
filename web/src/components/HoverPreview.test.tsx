// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { HoverPreview } from "./HoverPreview";
import { clearPreviewCache } from "../lib/preview";

afterEach(() => {
  cleanup();
  clearPreviewCache();
  vi.restoreAllMocks();
});

describe("HoverPreview", () => {
  it("fetches and shows a sanitized preview on link hover", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ markdown: "# Preview\n\nBody with <script>alert(1)</script>" }),
      }),
    ) as unknown as typeof fetch;

    render(
      <div className="doc-rendered">
        <a href="#/docs/context/b.md">Go B</a>
      </div>,
    );
    render(<HoverPreview delay={0} />);

    const link = screen.getByText("Go B");
    fireEvent.mouseOver(link);
    const tip = await screen.findByRole("tooltip");
    expect(tip.textContent).toContain("Preview");
    expect(tip.innerHTML).not.toContain("<script");
    // rendered in a root layer so panel overflow cannot clip it
    expect(tip.parentElement).toBe(document.body);
  });

  it("hides the preview when the pointer leaves the link", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ markdown: "# Preview" }) }),
    ) as unknown as typeof fetch;

    render(
      <div className="doc-rendered">
        <a href="#/docs/context/b.md">Go B</a>
      </div>,
    );
    render(<HoverPreview delay={0} />);

    const link = screen.getByText("Go B");
    fireEvent.mouseOver(link);
    await screen.findByRole("tooltip");
    fireEvent.mouseOut(link);
    await waitFor(() => expect(screen.queryByRole("tooltip")).toBeNull());
  });
});
