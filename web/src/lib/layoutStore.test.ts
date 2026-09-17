// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { clearLayout, loadLayout, resetLayout, saveLayout } from "./layoutStore";

afterEach(() => localStorage.clear());

describe("layoutStore", () => {
  it("round-trips a layout", () => {
    saveLayout("workspace", { panels: ["a", "b"] });
    expect(loadLayout("workspace")).toEqual({ panels: ["a", "b"] });
  });

  it("ignores a stale version and clears it", () => {
    localStorage.setItem("sdt-layout:workspace", JSON.stringify({ version: 0, layout: { x: 1 } }));
    expect(loadLayout("workspace")).toBeNull();
    expect(localStorage.getItem("sdt-layout:workspace")).toBeNull();
  });

  it("drops the previous layout schema version", () => {
    localStorage.setItem("sdt-layout:workspace", JSON.stringify({ version: 1, layout: { x: 1 } }));
    expect(loadLayout("workspace")).toBeNull();
    expect(localStorage.getItem("sdt-layout:workspace")).toBeNull();
  });

  it("recovers from corrupt JSON", () => {
    localStorage.setItem("sdt-layout:workspace", "{not json");
    expect(loadLayout("workspace")).toBeNull();
    expect(localStorage.getItem("sdt-layout:workspace")).toBeNull();
  });

  it("clears a stored layout", () => {
    saveLayout("detail", { a: 1 });
    clearLayout("detail");
    expect(loadLayout("detail")).toBeNull();
  });

  it("resetLayout clears the layout and triggers the reload", () => {
    saveLayout("workspace", { panels: ["a"] });
    const reload = vi.fn();
    resetLayout("workspace", reload);
    expect(loadLayout("workspace")).toBeNull();
    expect(reload).toHaveBeenCalledTimes(1);
  });
});
