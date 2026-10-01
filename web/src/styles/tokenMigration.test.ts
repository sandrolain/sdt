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
