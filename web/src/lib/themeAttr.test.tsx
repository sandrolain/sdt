// @vitest-environment jsdom
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { readThemeAttr, subscribeThemeAttr, useDocumentTheme } from "./themeAttr";

afterEach(() => {
  delete document.documentElement.dataset.theme;
  vi.unstubAllGlobals();
});

describe("readThemeAttr", () => {
  it("reads the theme the document declares, and null when it declares none", () => {
    expect(readThemeAttr()).toBeNull();
    document.documentElement.dataset.theme = "dark";
    expect(readThemeAttr()).toBe("dark");
    document.documentElement.dataset.theme = "light";
    expect(readThemeAttr()).toBe("light");
    document.documentElement.dataset.theme = "sepia";
    expect(readThemeAttr()).toBeNull();
  });
});

describe("subscribeThemeAttr", () => {
  it("notifies when the document's theme attribute changes", async () => {
    const onChange = vi.fn();
    const off = subscribeThemeAttr(onChange);
    act(() => {
      document.documentElement.dataset.theme = "dark";
    });
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    off();
    onChange.mockClear();
    act(() => {
      document.documentElement.dataset.theme = "light";
    });
    // The subscription is gone: nothing is reported through it.
    await Promise.resolve();
    expect(onChange).not.toHaveBeenCalled();
  });
});

describe("useDocumentTheme", () => {
  it("follows the document's attribute", () => {
    document.documentElement.dataset.theme = "dark";
    const { result } = renderHook(() => useDocumentTheme());
    expect(result.current).toBe("dark");
  });

  it("falls back to the system preference before the attribute is set", () => {
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({ matches: true }) as unknown as MediaQueryList),
    );
    const { result } = renderHook(() => useDocumentTheme());
    expect(result.current).toBe("dark");
  });
});
