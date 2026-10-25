import { describe, expect, it } from "vitest";
import {
  HIGHLIGHT_MAX_CHARS,
  HIGHLIGHT_MAX_LINES,
  highlightedToolOutput,
  toolOutputLanguage,
} from "./toolOutput";

describe("toolOutputLanguage", () => {
  it("reads an object or an array that parses as JSON", () => {
    expect(toolOutputLanguage('{"exitCode":0}')).toBe("json");
    expect(toolOutputLanguage("[1,2,3]")).toBe("json");
  });

  it("keeps a brace that is not a JSON document out of the json branch", () => {
    expect(toolOutputLanguage("{not json")).toBe("plaintext");
  });

  it("reads a unified diff from its own headers", () => {
    expect(toolOutputLanguage("diff --git a/x b/x\n--- a/x\n+++ b/x")).toBe("diff");
    expect(toolOutputLanguage("@@ -1,2 +1,2 @@\n-a\n+b")).toBe("diff");
  });

  it("leaves a test log and ordinary stdout as plaintext", () => {
    expect(toolOutputLanguage("ok  github.com/sandrolain/sdt/internal/acpbridge")).toBe(
      "plaintext",
    );
  });
});

describe("highlightedToolOutput", () => {
  it("escapes the output rather than injecting it", () => {
    const { html } = highlightedToolOutput("<script>alert(1)</script>");
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
  });

  it("keeps an over-long output plaintext, by characters and by lines", () => {
    expect(highlightedToolOutput('{"a":1}').language).toBe("json");
    expect(highlightedToolOutput("x".repeat(HIGHLIGHT_MAX_CHARS + 1)).language).toBe("plaintext");
    const lines = Array.from({ length: HIGHLIGHT_MAX_LINES + 1 }, () => "line").join("\n");
    expect(highlightedToolOutput(lines).language).toBe("plaintext");
  });
});
