// @vitest-environment node
import { describe, expect, it } from "vitest";
import { initialOpenDocs, OPEN_DOCS_CAP, openDocsReducer, type OpenDocsState } from "./openDocs";

function openMany(paths: string[]): OpenDocsState {
  return paths.reduce(
    (state, path) => openDocsReducer(state, { type: "open", path }),
    initialOpenDocs,
  );
}

describe("openDocsReducer", () => {
  it("appends on a route change to an unopened doc", () => {
    const s = openDocsReducer(openMany(["a", "b"]), { type: "route", path: "c" });
    expect(s).toEqual({ docs: ["a", "b", "c"], active: "c", seen: ["a", "b", "c"] });
  });

  it("activates an already-open doc on a route change", () => {
    const s = openDocsReducer(openMany(["a", "b"]), { type: "route", path: "a" });
    expect(s.docs).toEqual(["a", "b"]);
    expect(s.active).toBe("a");
  });

  it("appends new docs, activates them and marks them seen", () => {
    const s = openMany(["a", "b"]);
    expect(s.docs).toEqual(["a", "b"]);
    expect(s.active).toBe("b");
    expect(s.seen).toEqual(["a", "b"]);
  });

  it("re-activates an already open doc without reordering", () => {
    let s = openMany(["a", "b"]);
    s = openDocsReducer(s, { type: "activate", path: "a" });
    expect(s.docs).toEqual(["a", "b"]);
    expect(s.active).toBe("a");
  });

  it("closes the active doc and falls back to the previous one", () => {
    let s = openMany(["a", "b", "c"]);
    s = openDocsReducer(s, { type: "close", path: "c" });
    expect(s.docs).toEqual(["a", "b"]);
    expect(s.active).toBe("b");
  });

  it("closes a background doc without touching the active one", () => {
    let s = openMany(["a", "b", "c"]);
    s = openDocsReducer(s, { type: "close", path: "a" });
    expect(s.docs).toEqual(["b", "c"]);
    expect(s.active).toBe("c");
  });

  it("evicts the oldest doc beyond the cap, never the active one", () => {
    const paths = Array.from({ length: OPEN_DOCS_CAP + 2 }, (_, i) => `d${i}`);
    const s = openMany(paths);
    expect(s.docs).toHaveLength(OPEN_DOCS_CAP);
    expect(s.docs).not.toContain("d0");
    expect(s.docs).not.toContain("d1");
    expect(s.active).toBe(`d${OPEN_DOCS_CAP + 1}`);
  });

  it("closes every doc", () => {
    const s = openDocsReducer(openMany(["a", "b"]), { type: "closeAll" });
    expect(s.docs).toEqual([]);
    expect(s.active).toBeNull();
  });

  it("prunes docs missing from the corpus and fixes the active pointer", () => {
    let s = openMany(["a", "b", "c"]);
    s = openDocsReducer(s, { type: "prune", paths: ["a", "c"] });
    expect(s.docs).toEqual(["a", "c"]);
    expect(s.active).toBe("c");
    expect(s.seen).toEqual(["a", "c"]);
  });

  it("keeps the active doc when pruning leaves it in the corpus", () => {
    let s = openMany(["a", "b"]);
    s = openDocsReducer(s, { type: "activate", path: "a" });
    s = openDocsReducer(s, { type: "prune", paths: ["a", "b", "z"] });
    expect(s.docs).toEqual(["a", "b"]);
    expect(s.active).toBe("a");
  });
});
