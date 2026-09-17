import { describe, expect, it } from "vitest";
import { imageUrl } from "./images";

describe("imageUrl", () => {
  it("passes absolute URLs through", () => {
    expect(imageUrl("https://example.com/a.png")).toBe("https://example.com/a.png");
  });

  it("keeps corpus-relative paths", () => {
    expect(imageUrl("context/assets/a.png")).toBe("/api/file?path=context%2Fassets%2Fa.png");
  });

  it("resolves a document-relative path against the document directory", () => {
    expect(imageUrl("assets/a.png", "context/wiki/backend/auth.md")).toBe(
      "/api/file?path=context%2Fwiki%2Fbackend%2Fassets%2Fa.png",
    );
  });

  it("normalizes parent segments", () => {
    expect(imageUrl("../assets/a.png", "context/wiki/backend/auth.md")).toBe(
      "/api/file?path=context%2Fwiki%2Fassets%2Fa.png",
    );
  });

  it("strips quotes and returns empty for a blank value", () => {
    expect(imageUrl('"context/assets/a.png"')).toBe("/api/file?path=context%2Fassets%2Fa.png");
    expect(imageUrl("  ")).toBe("");
  });
});
