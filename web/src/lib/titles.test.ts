import { describe, expect, it } from "vitest";
import {
  displayTitle,
  fallbackTitle,
  firstH1,
  formatFilename,
  frontmatterTitle,
  stripDatePrefix,
  unwrapQuotes,
} from "./titles";

describe("frontmatterTitle", () => {
  it("extracts an unquoted title", () => {
    expect(frontmatterTitle("---\nkind: wiki\ntitle: My Doc\n---\n# Body")).toBe("My Doc");
  });

  it("strips surrounding quotes", () => {
    expect(frontmatterTitle('---\ntitle: "Quoted Title"\n---\n')).toBe("Quoted Title");
    expect(frontmatterTitle("---\ntitle: 'Single'\n---\n")).toBe("Single");
  });

  it("returns empty when missing", () => {
    expect(frontmatterTitle("---\nkind: wiki\n---\n")).toBe("");
    expect(frontmatterTitle(undefined)).toBe("");
  });
});

describe("fallbackTitle", () => {
  it("uses the basename without the .md extension", () => {
    expect(fallbackTitle("context/wiki/alpha.md")).toBe("alpha");
    expect(fallbackTitle("context/wiki/topic.map.md")).toBe("topic.map");
  });

  it("handles a trailing slash or non-md path", () => {
    expect(fallbackTitle("context/")).toBe("context");
    expect(fallbackTitle("notes")).toBe("notes");
  });
});

describe("unwrapQuotes / firstH1", () => {
  it("strips a wrapping quote pair", () => {
    expect(unwrapQuotes('"Quoted"')).toBe("Quoted");
    expect(unwrapQuotes("'Single'")).toBe("Single");
    expect(unwrapQuotes("plain")).toBe("plain");
  });

  it("extracts the first H1 and strips quotes", () => {
    expect(firstH1("intro\n# Real Heading\ntext")).toBe("Real Heading");
    expect(firstH1('# "Quoted Heading"')).toBe("Quoted Heading");
  });

  it("returns empty when there is no H1", () => {
    expect(firstH1("## Not H1\nbody")).toBe("");
    expect(firstH1(undefined)).toBe("");
  });
});

describe("stripDatePrefix / formatFilename", () => {
  it("drops a real date-lead prefix", () => {
    expect(stripDatePrefix("20260915-195559-plan-foo")).toBe("plan-foo");
    expect(stripDatePrefix("20260915-plan-foo")).toBe("20260915-plan-foo");
  });

  it("formats a filename into a display title", () => {
    expect(formatFilename("context/plan/20260915-195559-viewer-fixes.md")).toBe("Viewer fixes");
    expect(formatFilename("context/wiki/alpha.md")).toBe("Alpha");
    expect(formatFilename("context/plan/20260915-195559-viewer-fixes")).toBe("Viewer fixes");
  });

  it("strips quotes and keeps non-date hyphens", () => {
    expect(formatFilename('context/wiki/"two-words".md')).toBe("Two words");
  });
});

describe("displayTitle", () => {
  it("prefers an explicit title", () => {
    expect(displayTitle({ title: "Frontmatter", markdown: "# H1", path: "context/a.md" })).toBe(
      "Frontmatter",
    );
  });

  it("falls back to the first H1", () => {
    expect(displayTitle({ title: "", markdown: "# H1 Title", path: "context/a.md" })).toBe(
      "H1 Title",
    );
  });

  it("falls back to the formatted filename", () => {
    expect(
      displayTitle({ markdown: "no heading", path: "context/20260915-195559-foo-bar.md" }),
    ).toBe("Foo bar");
  });

  it("returns an empty string without any source", () => {
    expect(displayTitle({})).toBe("");
  });
});
