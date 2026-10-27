/**
 * Mounts the three Canvas 2D pets at the band's real size in both themes.
 *
 * The page is a measurement, not an app: it renders the characters, lists the modules the
 * 2D path actually loaded (the point of the family split is that `three` is not among
 * them), and samples the frame rate of the heaviest one.
 */
import { createPet2D } from "../src/lib/pet2d/createPet";
import type { Pet2DVariant } from "../src/lib/pet/types";

const VARIANTS: Pet2DVariant[] = ["fiamma", "scriptor", "possibile"];
const STATES = ["working", "speaking", "listening"] as const;
const THEMES = ["dark", "light"] as const;

function mount(host: HTMLElement, variant: Pet2DVariant, theme: "dark" | "light", state: string) {
  const tile = document.createElement("section");
  tile.className = "tile";
  tile.dataset.variant = variant;
  const surface = theme === "dark" ? "#070b09" : "#f7f8fc";
  const ink = theme === "dark" ? "#c8f7dc" : "#101a14";
  tile.style.background = surface;
  tile.style.color = ink;
  tile.dataset.theme = theme;
  tile.innerHTML = `<header><span>${variant}</span><span>${state} · ${theme}</span></header><div class="stage"></div>`;
  const stage = tile.querySelector<HTMLElement>(".stage")!;
  const canvas = document.createElement("canvas");
  canvas.style.cssText = "position:absolute;inset:0;width:100%;height:100%;display:block";
  stage.appendChild(canvas);
  host.appendChild(tile);

  const pet = createPet2D(variant, canvas, { density: 0.6, pixelRatioCap: 1.5 }, theme);
  pet.resize(400, 176);
  pet.setState(state as Parameters<typeof pet.setState>[0]);
  // One synchronous settled frame first: a browser panel may throttle requestAnimationFrame
  // for a tab it considers hidden, and a screenshot of a blank canvas would prove nothing.
  pet.setStill(true);
  pet.setStill(false);
  return pet;
}

const mounted: ReturnType<typeof mount>[] = [];
for (const theme of THEMES) {
  const row = document.getElementById(theme)!;
  VARIANTS.forEach((variant, i) => {
    mounted.push(mount(row, variant, theme, STATES[i]));
  });
}

// The draw cost of the heaviest character, measured synchronously: a browser panel may
// throttle requestAnimationFrame for a tab it considers hidden, but a frame's work is still
// a frame's work. 300 frames at 1/60 s, in the heaviest state.
const heavy = mounted[0] as unknown as { tick(dt: number): void };
const startedAt = performance.now();
for (let i = 0; i < 300; i++) heavy.tick(1 / 60);
const perFrame = (performance.now() - startedAt) / 300;

// The modules the page actually pulled: the 2D path must not have fetched `three`.
const report = document.getElementById("report")!;
const resources = performance
  .getEntriesByType("resource")
  .map((e) => e.name)
  .filter((name) => /\/three|three\.module|\/pet\/variants/.test(name));
const fpsLine = "fiamma @ 400×176, density 0.6, dpr " + window.devicePixelRatio + ": measuring…";

function render(fps: string) {
  report.textContent = [
    `three modules loaded by the 2D path: ${resources.length ? resources.join(", ") : "none"}`,
    fps || fpsLine,
    `pets mounted: ${mounted.length}`,
    `fiamma draw cost (working, 400×176, density 0.6): ${perFrame.toFixed(2)} ms/frame over 300 frames`,
  ].join("\n");
}
render("");

let frames = 0;
const started = performance.now();
const sample = () => {
  frames++;
  const elapsed = performance.now() - started;
  if (elapsed < 2_000) {
    requestAnimationFrame(sample);
    return;
  }
  render(
    `fiamma @ 400×176, density 0.6, dpr ${window.devicePixelRatio}: ${((frames / elapsed) * 1_000).toFixed(1)} fps over ${frames} frames`,
  );
};
setTimeout(() => {
  requestAnimationFrame(sample);
  // A throttled rAF never reaches the sample's end: say so rather than leave "measuring…".
  setTimeout(() => {
    if (frames === 0)
      render(
        "fiamma: requestAnimationFrame is throttled in this browser panel; the pixel check is the screenshot",
      );
  }, 3_000);
}, 400);
