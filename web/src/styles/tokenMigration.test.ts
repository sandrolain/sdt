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

/**
 * Token vocabulary guard (phase 6 of the agent auto-start wave). A custom property that
 * does not exist is not an error anywhere: the declaration is invalid at computed-value
 * time and the property keeps its initial value, which is how the agent panel shipped
 * transparent — `background: var(--bg)` where the token layer declares `--bg-base`,
 * `--bg-mantle` and `--bg-crust`, and `--bg` exists only inside the JsonCanvas scope,
 * injected by `web/src/lib/jsoncanvas/theme.ts` at runtime.
 *
 * So this guard is the mechanical part of "the panel is opaque": every custom property the
 * component stylesheet refers to is either declared in the token layer or named here as
 * belonging to a scope that injects its own.
 */
describe("the component stylesheet only refers to custom properties that exist", () => {
  // Scopes that set their variables on their own element at runtime, told apart by their
  // selectors: the JSON Canvas container (`lib/jsoncanvas/theme.ts`) and the dockview theme
  // (`lib/dockLayout.ts`). The exemption is **per rule** rather than a global list of names,
  // because a global list is exactly how this guard would have accepted `var(--bg)` in the
  // agent panel — the defect it exists for: `--bg` is real inside the canvas and nowhere
  // else.
  const injectedScopes = [".jc", ".dv-", ".dock-layout", "[cmdk-"];

  it("declares or accounts for every bare var() the component stylesheet uses", () => {
    // A property declared in this stylesheet is a local variable of the surface that owns
    // it (the callout colour is the one), which is as real as a token-layer one.
    const declared = new Set([
      ...[...tokens.matchAll(/(--[a-z0-9-]+)\s*:/g)].map((m) => m[1]),
      ...[...css.matchAll(/(--[a-z0-9-]+)\s*:/g)].map((m) => m[1]),
    ]);

    // A leaf block only: a body that contains braces is an enclosing block, and its rules
    // are matched on their own. A reference *with* a fallback is exempt too — it degrades to
    // the fallback rather than rendering nothing, which is the defect this is about.
    const offenders: string[] = [];
    for (const block of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
      const context = block[1].replace(/\s+/g, " ").trim();
      if (injectedScopes.some((scope) => context.includes(scope))) continue;
      for (const reference of block[2].matchAll(/var\((--[a-z0-9-]+)\)/g)) {
        if (!declared.has(reference[1])) offenders.push(`${context} → ${reference[1]}`);
      }
    }
    expect(offenders).toEqual([]);
  });
});
