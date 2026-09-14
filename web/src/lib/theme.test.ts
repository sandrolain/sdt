import { describe, expect, it } from "vitest";
import { nextTheme, pickTheme } from "./theme";

describe("pickTheme", () => {
  it("honors explicit light and dark preferences", () => {
    expect(pickTheme("light", true)).toBe("light");
    expect(pickTheme("dark", false)).toBe("dark");
  });

  it("resolves system preference from the OS dark flag", () => {
    expect(pickTheme("system", true)).toBe("dark");
    expect(pickTheme("system", false)).toBe("light");
  });
});

describe("nextTheme", () => {
  it("cycles light → dark → system → light", () => {
    expect(nextTheme("light")).toBe("dark");
    expect(nextTheme("dark")).toBe("system");
    expect(nextTheme("system")).toBe("light");
  });
});
