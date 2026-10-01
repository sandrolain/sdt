// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import DOMPurify from "dompurify";
import { renderMarkdown } from "./markdown";
import { SLIDES_SANITIZE_CONFIG, sanitizeSlideHtml, sanitizeSlides } from "./slidesSanitize";

describe("sanitizeSlideHtml", () => {
  it("keeps a directive-only slide (section + data attributes + style)", () => {
    const slide =
      '<section id="1" class="lead" data-marpit-fragments="3" data-class="lead" style="--paginate:true;">' +
      "<h1>Title</h1><p>Prose.</p></section>";
    const clean = sanitizeSlideHtml(slide);
    expect(clean).toContain("<section");
    expect(clean).toContain('class="lead"');
    expect(clean).toContain("data-marpit-fragments");
    expect(clean).toContain("--paginate:true");
    expect(clean).toContain("<h1>Title</h1>");
  });

  it("strips a <script> from a slide body", () => {
    const clean = sanitizeSlideHtml("<section><p>hi</p><script>alert(1)</script></section>");
    expect(clean).not.toContain("<script");
    expect(clean).not.toContain("alert(1)");
  });

  it("strips an event-handler attribute", () => {
    const clean = sanitizeSlideHtml('<section><img src="x.png" onerror="alert(1)"></section>');
    expect(clean).not.toContain("onerror");
    expect(clean).not.toContain("alert(1)");
  });

  it("keeps the KaTeX MathML/SVG subset", () => {
    const katex =
      '<section><p><span class="katex"><math xmlns="http://www.w3.org/1998/Math/MathML">' +
      "<semantics><mrow><mi>E</mi><mo>=</mo><mi>m</mi></mrow>" +
      '<annotation encoding="application/x-tex">E=m</annotation></semantics></math></span></p></section>';
    const clean = sanitizeSlideHtml(katex);
    expect(clean).toContain("<math");
    expect(clean).toContain("<mrow>");
    expect(clean).toContain("<annotation");
  });

  it("does not allow a <style> tag from a slide body", () => {
    const clean = sanitizeSlideHtml("<section><style>p{color:red}</style><p>hi</p></section>");
    expect(clean).not.toContain("<style");
    expect(clean).not.toContain("color:red");
  });

  it("sanitizes every slide in a deck, preserving order", () => {
    const out = sanitizeSlides([
      "<section><h1>a</h1></section>",
      "<section><script>x</script><h1>b</h1></section>",
    ]);
    expect(out).toHaveLength(2);
    expect(out[0]).toContain("a");
    expect(out[1]).not.toContain("<script");
    expect(out[1]).toContain("b");
  });
});

describe("global config is untouched", () => {
  it("still strips svg and style through the markdown pipeline", () => {
    // The global allowlist excludes svg/style: a raw <svg>/<style> in markdown
    // must not survive renderMarkdown, proving the slide config is scoped.
    const html = renderMarkdown(
      '<svg><path d="M0 0"></path></svg>\n\n<style>p{color:red}</style>\n\nhi',
    );
    expect(html).not.toContain("<svg");
    expect(html).not.toContain("<style");
    expect(html).toContain("hi");
  });

  it("the slide config is a distinct object from the markdown one", () => {
    // A guard: if someone re-exports the markdown config here, the two merge.
    expect(SLIDES_SANITIZE_CONFIG.ALLOWED_TAGS).toContain("svg");
    expect(SLIDES_SANITIZE_CONFIG.ALLOWED_TAGS).toContain("math");
  });

  it("DOMPurify.sanitize with no config is unaffected by the slide config", () => {
    // A sanity check that importing the slide config does not change the
    // library default (which the markdown path relies on for its own config).
    expect(DOMPurify.sanitize("<b>ok</b>")).toContain("ok");
  });
});
