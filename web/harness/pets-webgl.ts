/**
 * The WebGL pets at the stage size the panel uses (418×164) and in a square box, so a shape
 * can be judged against the box it is drawn in. It also prints each canvas' buffer and CSS
 * size: a mismatch is the "stretched" look, and this page is where that is visible.
 */
import { createPet } from "../src/lib/pet/createPet";
import type { PetState, PetWebGLVariant } from "../src/lib/pet/types";

const VARIANTS: PetWebGLVariant[] = ["dot", "sciame", "trama"];
const STATES: PetState[] = ["thinking", "working", "speaking"];

const row = document.getElementById("row")!;
const report = document.createElement("div");
report.style.cssText = "margin-top:12px;white-space:pre;font-size:12px";
document.body.appendChild(report);

interface Mounted {
  canvas: HTMLCanvasElement;
  w: number;
  h: number;
}

function mountAt(variant: PetWebGLVariant, state: PetState, w: number, h: number): Mounted {
  const tile = document.createElement("section");
  tile.className = "tile";
  tile.style.width = `${w}px`;
  tile.innerHTML = `<header><span>${variant}</span><span>${state} · ${w}×${h}</span></header><div class="stage" style="block-size:${h}px"></div>`;
  const stage = tile.querySelector<HTMLElement>(".stage")!;
  const canvas = document.createElement("canvas");
  canvas.style.cssText = "position:absolute;inset:0;width:100%;height:100%;display:block";
  stage.appendChild(canvas);
  row.appendChild(tile);
  const pet = createPet(variant, canvas, { density: 0.6, pixelRatioCap: 1.5 });
  pet.resize(w, h);
  pet.setState(state);
  pet.setStill(true);
  return { canvas, w, h };
}

const mounted: Mounted[] = [];
for (const [w, h] of [
  [418, 164],
  [164, 164],
] as const) {
  VARIANTS.forEach((variant, i) => mounted.push(mountAt(variant, STATES[i], w, h)));
}

setTimeout(() => {
  report.textContent = mounted
    .map(({ canvas, w, h }) => {
      const r = canvas.getBoundingClientRect();
      return `buffer ${canvas.width}×${canvas.height}  css ${Math.round(r.width)}×${Math.round(r.height)}  asked ${w}×${h}  visible ${canvas.clientWidth}×${canvas.clientHeight}`;
    })
    .join("\n");
}, 600);
