// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { clearPreviewCache, loadPreview, previewHtml, previewPathFromHref } from "./preview";

afterEach(() => {
  clearPreviewCache();
  vi.restoreAllMocks();
});

describe("previewPathFromHref", () => {
  it("resolves docs and wiki routes", () => {
    expect(previewPathFromHref("#/docs/context/wiki/a.md")).toBe("context/wiki/a.md");
    expect(previewPathFromHref("#/wiki/backend")).toBe("context/wiki/backend.md");
  });

  it("ignores non-document links", () => {
    expect(previewPathFromHref("https://example.com")).toBeNull();
    expect(previewPathFromHref("#/wiki/graph")).toBeNull();
    expect(previewPathFromHref("#/wiki/board")).toBeNull();
  });
});

describe("previewHtml", () => {
  it("strips frontmatter and clips long bodies", () => {
    const md = "---\ntitle: X\n---\n\n# Heading\n\nbody text";
    const html = previewHtml(md);
    expect(html).not.toContain("title: X");
    expect(html).toContain("Heading");
    expect(previewHtml(`---\n---\n${"a".repeat(3000)}`)).toContain("…");
  });
});

describe("loadPreview", () => {
  it("fetches once and caches by path", async () => {
    const fetchMock = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ markdown: "# Cached" }) }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    const controller = new AbortController();
    const first = await loadPreview("context/a.md", controller.signal);
    const second = await loadPreview("context/a.md", controller.signal);
    expect(first).toBe(second);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(first).toContain("Cached");
  });

  it("raises on a failed fetch without caching", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: false, status: 404, json: () => Promise.resolve({}) }),
    ) as unknown as typeof fetch;
    const controller = new AbortController();
    await expect(loadPreview("context/missing.md", controller.signal)).rejects.toThrow("404");
  });
});
