/// <reference types="node" />
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

/**
 * Token migration guard (phase 9 of plan 20260929-201338): the elevation shadow
 * is a semantic role, so its colour lives in the token layer. The categorical
 * palettes (graph clusters, edges, boundary accents) are identities and stay raw
 * by decision — this guard protects the role, not the palettes.
 */

const css = readFileSync(new URL("./index.css", import.meta.url), "utf8");
const tokens = readFileSync(new URL("./tokens.css", import.meta.url), "utf8");

describe("token layer owns the elevation shadow", () => {
  it("declares both shadow strengths", () => {
    expect(tokens).toContain("--shadow-color:");
    expect(tokens).toContain("--shadow-color-soft:");
  });

  it("keeps no raw shadow colour in the component stylesheet", () => {
    expect(css).not.toMatch(/rgba?\(\s*0\s*,\s*0\s*,\s*0\s*,/);
    expect(css).not.toMatch(/rgb\(\s*0\s+0\s+0\s*\//);
  });

  it("casts every cast elevation shadow with the token", () => {
    // an inset shadow is a marker or a highlight, not an elevation
    const cast = [...css.matchAll(/box-shadow:\s*([^;]+);/g)]
      .map((m) => m[1])
      .filter((shadow) => !shadow.trim().startsWith("inset"));
    expect(cast.length).toBeGreaterThan(0);
    for (const shadow of cast) {
      expect(shadow).toContain("var(--shadow-color");
    }
  });
});

/**
 * Corner-radius guard (phase 5 of plan 20261010-125656). `tokens.css` owns the
 * scale; a radius that is a semantic role (card, chip, control, pill) uses a
 * token. A radius off the scale — a hairline data bar, a categorical swatch, a
 * one-off tab — stays raw, but only with a one-line comment saying why. This
 * makes the convention in `tokens.css` a gate rather than prose: a new raw
 * radius has to justify itself at its own site.
 */

const SCALE_PX = [4, 6, 8, 10, 12, 999];

const STYLESHEETS = [
  { name: "index.css", css },
  {
    name: "JsonCanvas.css",
    css: readFileSync(new URL("../components/JsonCanvas.css", import.meta.url), "utf8"),
  },
];

describe("token layer owns the corner-radius scale", () => {
  it("declares the scale", () => {
    for (const step of [
      "--radius-xs",
      "--radius-sm",
      "--radius-md",
      "--radius-lg",
      "--radius-xl",
    ]) {
      expect(tokens).toContain(`${step}:`);
    }
    expect(tokens).toContain("--radius-pill:");
  });

  it.each(STYLESHEETS)("$name has no raw radius on the scale", ({ css: sheet }) => {
    const onScale = [...sheet.matchAll(/border-radius:\s*([^;]+);/g)].filter(
      ([, value]) =>
        // a var() is a token under another name (e.g. the map node radius); a raw
        // literal whose corner sits on the scale belongs to the scale
        !value.includes("var(") &&
        value.split(/\s+/).some((corner) => SCALE_PX.includes(Number.parseInt(corner, 10))),
    );
    expect(onScale.map(([, value]) => value)).toEqual([]);
  });

  it.each(STYLESHEETS)(
    "$name documents every radius left off the scale",
    ({ name, css: sheet }) => {
      const lines = sheet.split("\n");
      const undocumented: string[] = [];
      lines.forEach((line, i) => {
        if (!/border-radius:\s*[^;]+;/.test(line)) return;
        if (line.includes("var(")) return;
        const previous = lines[i - 1]?.trim() ?? "";
        if (!(previous.startsWith("/*") && previous.endsWith("*/"))) {
          undocumented.push(`${name}:${i + 1}: ${line.trim()}`);
        }
      });
      expect(undocumented).toEqual([]);
    },
  );
});
