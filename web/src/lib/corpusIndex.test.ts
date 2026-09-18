// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  hrefCorpusPath,
  linkKind,
  loadCorpusIndex,
  resetCorpusIndexCache,
  type CorpusIndex,
} from "./corpusIndex";
import type { TreeEntry } from "./api";
import { kindFromPath } from "./kinds";

afterEach(() => {
  vi.restoreAllMocks();
  resetCorpusIndexCache();
});

describe("hrefCorpusPath", () => {
  it("maps docs and wiki hash routes to corpus paths", () => {
    expect(hrefCorpusPath("/docs/context/plan/a.md")).toBe("context/plan/a.md");
    expect(hrefCorpusPath("/wiki/sdt-dev-lifecycle")).toBe("context/wiki/sdt-dev-lifecycle.md");
    expect(hrefCorpusPath("https://example.com")).toBeNull();
  });
});

describe("kindFromPath", () => {
  it("derives the kind from the corpus folder", () => {
    expect(kindFromPath("context/analysis/a.md")).toBe("analysis");
    expect(kindFromPath("context/decisions/0001-x.md")).toBe("decision");
    expect(kindFromPath("context/wiki/x.md")).toBe("wiki");
    expect(kindFromPath("context/unknown/x.md")).toBe("other");
  });
});

describe("loadCorpusIndex", () => {
  function mockTree(entries: unknown[]) {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ entries }) }),
    ) as unknown as typeof fetch;
  }

  it("builds a path → entry map and caches it", async () => {
    mockTree([
      { path: "context/plan/p.md", kind: "plan", title: "Plan" },
      { path: "context/board.canvas", kind: "canvas", canvas: true },
    ]);
    const index = await loadCorpusIndex();
    expect(index.get("context/plan/p.md")).toEqual({
      entry: { path: "context/plan/p.md", kind: "plan", title: "Plan" },
      kind: "plan",
    });
    expect(index.get("context/board.canvas")?.entry.kind).toBe("canvas");
    expect(index.get("context/board.canvas")?.kind).toBe("canvas");
    await loadCorpusIndex();
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
  });

  it("clears the cache when the request fails", async () => {
    globalThis.fetch = vi.fn(() =>
      Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({ error: "boom" }) }),
    ) as unknown as typeof fetch;
    await expect(loadCorpusIndex()).rejects.toThrow("boom");
    mockTree([]);
    await expect(loadCorpusIndex()).resolves.toBeInstanceOf(Map);
  });
});

describe("linkKind", () => {
  it("prefers the index kind and falls back to the folder", () => {
    const index: CorpusIndex = new Map([
      [
        "context/wiki/x.md",
        { entry: { path: "context/wiki/x.md", kind: "wiki" } as TreeEntry, kind: "wiki" },
      ],
    ]);
    expect(linkKind("/wiki/x", index)).toBe("wiki");
    expect(linkKind("/docs/context/notes/y.md", index)).toBe("notes");
    expect(linkKind("https://example.com", index)).toBeUndefined();
  });
});
