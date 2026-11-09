// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { resolveSlideAssets } from "./slidesAssets";

const DECK = "context/notes/deck.slide.md";

describe("resolveSlideAssets", () => {
  it("rewrites an inline image relative to the deck", () => {
    const out = resolveSlideAssets('<section><img src="../assets/pic.jpg" alt=""></section>', DECK);
    expect(out).toContain('src="/api/file?path=context%2Fassets%2Fpic.jpg"');
  });

  it("rewrites a background image in the section style and its data attribute", () => {
    const out = resolveSlideAssets(
      '<section style="background-image:url(&quot;../assets/bg.jpg&quot;);background-position:center;" ' +
        'data-background-image="url(&quot;../assets/bg.jpg&quot;)"></section>',
      DECK,
    );
    expect(out.match(/\/api\/file\?path=context%2Fassets%2Fbg\.jpg/g)).toHaveLength(2);
  });

  it("leaves remote, data and already-resolved values alone", () => {
    const html =
      '<section style="background-image:url(https://example.com/a.png);" data-background-image="url(data:image/png;base64,AAA)">' +
      '<img src="https://example.com/b.png"><img src="data:image/png;base64,BBB">' +
      '<img src="/api/file?path=context%2Fassets%2Fc.png"></section>';
    expect(resolveSlideAssets(html, DECK)).toBe(html);
  });

  it("returns the html untouched when there is no base path", () => {
    const html = '<section><img src="../assets/pic.jpg"></section>';
    expect(resolveSlideAssets(html)).toBe(html);
  });
});
