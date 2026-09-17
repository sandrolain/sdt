// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { clearPreviewCache, loadPreview, previewMeta, previewPathFromHref } from "./preview";

afterEach(() => {
  clearPreviewCache();
  vi.restoreAllMocks();
});

describe("previewPathFromHref", () => {
  it("resolves docs and wiki routes", () => {
    expect(previewPathFromHref("/docs/context/wiki/a.md")).toBe("context/wiki/a.md");
    expect(previewPathFromHref("/wiki/backend")).toBe("context/wiki/backend.md");
  });

  it("ignores non-document links", () => {
    expect(previewPathFromHref("https://example.com")).toBeNull();
    expect(previewPathFromHref("/wiki/graph")).toBeNull();
    expect(previewPathFromHref("/wiki/board")).toBeNull();
  });
});

describe("previewMeta", () => {
  it("extracts title, summary and dates from the frontmatter", () => {
    const fm = [
      "---",
      'title: "Quoted title"',
      "summary: Short summary",
      "created: 2026-09-01",
      "updated: 2026-09-10",
      "image: context/assets/cover.png",
      "---",
    ].join("\n");
    expect(previewMeta("context/notes/x.md", fm)).toEqual({
      title: "Quoted title",
      summary: "Short summary",
      created: "2026-09-01",
      modified: "2026-09-10",
      image: "context/assets/cover.png",
      path: "context/notes/x.md",
    });
  });

  it("falls back to the path-derived title", () => {
    expect(previewMeta("context/notes/x.md", undefined).title).toBe("X");
  });
});

describe("loadPreview", () => {
  it("fetches once and caches metadata by path", async () => {
    const fetchMock = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({ frontmatter: "---\ntitle: Cached\n---\n", markdown: "# Cached" }),
      }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    const controller = new AbortController();
    const first = await loadPreview("context/a.md", controller.signal);
    const second = await loadPreview("context/a.md", controller.signal);
    expect(first).toEqual(second);
    expect(first.title).toBe("Cached");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("raises on a failed fetch without caching", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) }),
    ) as unknown as typeof fetch;
    const controller = new AbortController();
    await expect(loadPreview("context/missing.md", controller.signal)).rejects.toThrow("404");
  });
});
