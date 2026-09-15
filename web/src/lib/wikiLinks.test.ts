import { describe, expect, it } from "vitest";
import type { TreeEntry } from "./api";
import {
  buildWikiIndex,
  docHref,
  parseWikiLink,
  resolveDocPath,
  resolveWikiLink,
  rewriteWikiLinks,
  wikiHref,
  wikiIdFromPath,
} from "./wikiLinks";

const ENTRIES: TreeEntry[] = [
  { path: "context/wiki/accounts.md", title: "Account Service" },
  { path: "context/wiki/sub/deep.md", title: "Deep Page" },
  { path: "context/wiki/topic.map.md", title: "Topic Map", isMap: true },
  { path: "context/wiki/board.canvas", canvas: true },
  { path: "context/notes/note.md", title: "Not wiki" },
];

describe("wikiIdFromPath", () => {
  it("strips the wiki prefix and .md suffix", () => {
    expect(wikiIdFromPath("context/wiki/accounts.md")).toBe("accounts");
    expect(wikiIdFromPath("context/wiki/sub/deep.md")).toBe("sub/deep");
    expect(wikiIdFromPath("context/wiki/topic.map.md")).toBe("topic.map");
  });
});

describe("buildWikiIndex", () => {
  it("indexes wiki pages by id and title, skipping canvas and non-wiki", () => {
    const ix = buildWikiIndex(ENTRIES);
    expect(ix.byId.get("accounts")).toBe("Account Service");
    expect(ix.byTitle.get("Deep Page")).toBe("sub/deep");
    expect(ix.byTitle.has("Not wiki")).toBe(false);
    expect(ix.byId.has("board")).toBe(false);
  });
});

describe("parseWikiLink", () => {
  it("parses id, id|label and verb::title forms", () => {
    expect(parseWikiLink("accounts")).toEqual({
      raw: "accounts",
      verb: undefined,
      target: "accounts",
      label: "accounts",
    });
    expect(parseWikiLink("accounts|Account Service")?.label).toBe("Account Service");
    expect(parseWikiLink("depends_on::Account Service")).toEqual({
      raw: "depends_on::Account Service",
      verb: "depends_on",
      target: "Account Service",
      label: "Account Service",
    });
  });

  it("rejects empty targets", () => {
    expect(parseWikiLink("  ")).toBeNull();
    expect(parseWikiLink("|label")).toBeNull();
  });
});

describe("resolveWikiLink", () => {
  const ix = buildWikiIndex(ENTRIES);

  it("resolves ids and titles through the index", () => {
    expect(resolveWikiLink(parseWikiLink("accounts")!, ix)).toBe("accounts");
    expect(resolveWikiLink(parseWikiLink("Account Service")!, ix)).toBe("accounts");
  });

  it("resolves verb links by title and leaves unknown verbs unresolved", () => {
    expect(resolveWikiLink(parseWikiLink("refers_to::Account Service")!, ix)).toBe("accounts");
    expect(resolveWikiLink(parseWikiLink("refers_to::Nope")!, ix)).toBeNull();
  });

  it("treats an unindexed id form as an id, but not an unindexed verb form", () => {
    expect(resolveWikiLink(parseWikiLink("future-page")!)).toBe("future-page");
    expect(resolveWikiLink(parseWikiLink("refers_to::Nope")!)).toBeNull();
  });
});

describe("resolveDocPath", () => {
  it("resolves relative links against the base document", () => {
    expect(resolveDocPath("./other.md", "context/wiki/accounts.md")).toBe("context/wiki/other.md");
    expect(resolveDocPath("../analysis/x.md", "context/wiki/accounts.md")).toBe(
      "context/analysis/x.md",
    );
    expect(resolveDocPath("/context/plan/p.md", "context/wiki/accounts.md")).toBe(
      "context/plan/p.md",
    );
  });
});

describe("hrefs", () => {
  it("builds hash routes", () => {
    expect(wikiHref("accounts")).toBe("#/wiki/accounts");
    expect(docHref("context/notes/n.md")).toBe("#/docs/context/notes/n.md");
  });
});

describe("rewriteWikiLinks", () => {
  const ix = buildWikiIndex(ENTRIES);

  it("rewrites resolved links to wiki routes", () => {
    expect(rewriteWikiLinks("see [[accounts|Account Service]]", ix)).toBe(
      "see [Account Service](#/wiki/accounts)",
    );
    expect(rewriteWikiLinks("see [[refers_to::Account Service]]", ix)).toBe(
      "see [Account Service](#/wiki/accounts)",
    );
  });

  it("renders unresolved links as inert broken spans", () => {
    const out = rewriteWikiLinks("see [[refers_to::Nope]]", ix);
    expect(out).toContain('class="wikilink-broken"');
    expect(out).not.toContain("#/wiki/");
  });

  it("leaves non-wikilink text untouched", () => {
    expect(rewriteWikiLinks("plain [text](a.md)", ix)).toBe("plain [text](a.md)");
  });
});
