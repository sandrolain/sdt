import { describe, expect, it } from "vitest";
import { safeExternalUrl, urlHost } from "./url";

describe("safeExternalUrl", () => {
  it("allows http and https", () => {
    expect(safeExternalUrl("https://example.com/a")).toBe("https://example.com/a");
    expect(safeExternalUrl("http://example.com")).toBe("http://example.com/");
  });

  it("rejects javascript, data and relative URLs", () => {
    expect(safeExternalUrl("javascript:alert(1)")).toBeNull();
    expect(safeExternalUrl("data:text/html,x")).toBeNull();
    expect(safeExternalUrl("/wiki/x")).toBeNull();
    expect(safeExternalUrl(undefined)).toBeNull();
  });
});

describe("urlHost", () => {
  it("returns the hostname or the raw value", () => {
    expect(urlHost("https://example.com/a")).toBe("example.com");
    expect(urlHost("not a url")).toBe("not a url");
  });
});
