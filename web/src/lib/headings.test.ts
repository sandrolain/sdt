import { describe, expect, it } from "vitest";
import { normalizeHeadingText, stripLeadingH1 } from "./headings";

describe("stripLeadingH1", () => {
  it("removes one leading top-level heading only", () => {
    expect(stripLeadingH1("# Title\n\n## Sub\n")).toBe("\n## Sub\n");
    expect(stripLeadingH1("## Sub\n")).toBe("## Sub\n");
  });
});

describe("normalizeHeadingText", () => {
  it("strips inline code spans", () => {
    expect(normalizeHeadingText("Option A: fold into `development.md`")).toBe(
      "Option A: fold into development.md",
    );
  });

  it("strips emphasis", () => {
    expect(normalizeHeadingText("**Bold** and *em* and _under_")).toBe("Bold and em and under");
  });

  it("keeps a link's text and drops the target", () => {
    expect(normalizeHeadingText("See [the docs](https://example.com/x) now")).toBe(
      "See the docs now",
    );
  });

  it("treats an image like a link (keeps the alt text)", () => {
    expect(normalizeHeadingText("![icon](a.svg) Title")).toBe("icon Title");
  });

  it("collapses whitespace and trims", () => {
    expect(normalizeHeadingText("  A   heading \n with\tspaces ")).toBe("A heading with spaces");
  });

  it("leaves a plain heading unchanged", () => {
    expect(normalizeHeadingText("Phase 1 — Find reset")).toBe("Phase 1 — Find reset");
  });
});
