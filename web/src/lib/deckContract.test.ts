import { describe, expect, it } from "vitest";
import { Marp } from "@marp-team/marp-core";

/**
 * The delivery contract of a deck: `material export` drops the machine header,
 * so the frontmatter is the first thing Marp sees. These two fixtures pin both
 * halves — the exported shape renders one section per slide with pagination, and
 * the authoring shape (a leading header before the frontmatter) does not.
 */

const EXPORTED = `---
theme: default
paginate: true
---

# Uno

Testo.

---

# Due

Testo.
`;

const AUTHORING = `<!-- material: concept=wiki/x; syllabus=syllabi/x; lang=it; classification=undeclared; claims=c-1@abc -->

---
theme: default
paginate: true
---

# Uno

Testo.
`;

function sections(html: string): number {
  return (html.match(/<section/g) ?? []).length;
}

describe("the exported deck contract", () => {
  it("renders one section per slide, pagination on, and no frontmatter text", () => {
    const { html } = new Marp().render(EXPORTED);
    expect(sections(html)).toBe(2);
    expect(html).not.toContain("theme: default");
    expect((html.match(/data-paginate/g) ?? []).length).toBe(2);
  });

  it("rejects the authoring shape: an empty first slide and the frontmatter as text", () => {
    const { html } = new Marp().render(AUTHORING);
    expect(html).toContain('<section id="1"></section>');
    expect(html).toContain("theme: default");
    expect((html.match(/data-paginate/g) ?? []).length).toBe(0);
  });
});
