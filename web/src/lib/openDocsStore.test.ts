// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { initialOpenDocs, loadOpenDocs, saveOpenDocs } from "./openDocs";

afterEach(() => localStorage.clear());

describe("openDocs persistence", () => {
  it("round-trips open docs and the active pointer", () => {
    saveOpenDocs({ docs: ["a", "b"], active: "a", seen: ["a", "b"] });
    expect(loadOpenDocs()).toEqual({ docs: ["a", "b"], active: "a", seen: ["a", "b"] });
  });

  it("returns null when nothing is stored", () => {
    expect(loadOpenDocs()).toBeNull();
  });

  it("clears the stored entry when the stack is emptied", () => {
    saveOpenDocs({ docs: ["a"], active: "a", seen: ["a"] });
    saveOpenDocs(initialOpenDocs);
    expect(loadOpenDocs()).toBeNull();
  });

  it("repairs an active pointer that is not part of the stack", () => {
    saveOpenDocs({ docs: ["a", "b"], active: "b", seen: [] });
    localStorage.setItem(
      "sdt-layout:open-docs",
      JSON.stringify({ version: 3, layout: { docs: ["a", "b"], active: "zzz" } }),
    );
    expect(loadOpenDocs()?.active).toBe("b");
  });

  it("ignores a corrupt stored shape", () => {
    localStorage.setItem(
      "sdt-layout:open-docs",
      JSON.stringify({ version: 3, layout: { docs: "nope" } }),
    );
    expect(loadOpenDocs()).toBeNull();
  });
});
