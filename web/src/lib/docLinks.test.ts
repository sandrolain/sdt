// @vitest-environment node
import { describe, expect, it } from "vitest";
import { buildWikiIndex, type WikiIndex } from "./wikiLinks";
import { collectBodyLinks, collectMetaLinks, resolveDocLink } from "./docLinks";

const INDEX: WikiIndex = buildWikiIndex([
  { path: "context/wiki/backend.md", title: "Backend" },
  { path: "context/wiki/api.md", title: "API" },
] as never);

describe("resolveDocLink", () => {
  it("resolves corpus paths to docs routes", () => {
    expect(resolveDocLink("context/analysis/a.md", "context/wiki/x.md")).toEqual({
      label: "A",
      href: "/docs/context/analysis/a.md",
    });
    expect(resolveDocLink("plan/20260915-195559-foo-bar.md", "context/x.md")).toEqual({
      label: "Foo bar",
      href: "/docs/context/plan/20260915-195559-foo-bar.md",
    });
  });

  it("marks excluded targets name-only", () => {
    const link = resolveDocLink("context/instructions/plan.md", "context/x.md");
    expect(link.href).toBeUndefined();
    expect(link.excluded).toBe(true);
    expect(link.label).toBe("Plan");
  });

  it("resolves wikilinks through the index", () => {
    expect(resolveDocLink("[[backend]]", "context/wiki/x.md", INDEX)).toEqual({
      label: "Backend",
      href: "/wiki/backend",
    });
    expect(resolveDocLink("[[Unknown]]", "context/wiki/x.md", INDEX)).toEqual({
      label: "Unknown",
    });
  });

  it("flags external links", () => {
    expect(resolveDocLink("https://example.com", "context/x.md")).toEqual({
      label: "https://example.com",
      href: "https://example.com",
      external: true,
    });
  });
});

describe("collectMetaLinks / collectBodyLinks", () => {
  it("collects links/sources/relations only", () => {
    const links = collectMetaLinks(
      [
        { key: "sources", values: ["analysis/a.md"] },
        { key: "title", values: ["Title"] },
      ],
      "context/x.md",
    );
    expect(links).toHaveLength(1);
    expect(links[0].href).toBe("/docs/context/analysis/a.md");
  });

  it("collects body wikilinks and md links", () => {
    const links = collectBodyLinks(
      "see [[backend]] and [x](notes/n.md)",
      "context/wiki/x.md",
      INDEX,
    );
    expect(links.map((l) => l.href)).toEqual(["/wiki/backend", "/docs/context/notes/n.md"]);
  });
});
