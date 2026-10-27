/**
 * The WebGL family's base: one renderer, one scene, one camera, on the shared clock.
 *
 * Vendored from `context/refs/react/pet-react/PetBase.ts` (this project's own material),
 * with comments translated and two additions the panel needs: a still mode, so reduced
 * motion and a resting pet get one settled frame instead of a loop, and a disposal path
 * that is safe to call once per mounted strip.
 *
 * The state machine, the still/paused paths and the clock were extracted to `core.ts` when
 * the Canvas 2D family arrived, so both families run on one implementation. What is left
 * here is the renderer: a 2D character cannot build one, and the three variants must not
 * change.
 */
import * as THREE from "three";
import { PetCore } from "./core";
import type { PetOptions } from "./types";

/**
 * Builds a Color without a colour-space conversion. The variants were drawn with "raw"
 * values (as in three r128, before colour management): with
 * `renderer.outputColorSpace = LinearSRGBColorSpace` and this helper the look stays
 * identical to the prototypes.
 */
export const col = (c: string | number): THREE.Color => {
  const k = new THREE.Color();
  if (typeof c === "string") k.setStyle(c, THREE.LinearSRGBColorSpace);
  else k.setHex(c, THREE.LinearSRGBColorSpace);
  return k;
};

export const WHITE = col(0xffffff);

export abstract class PetBase extends PetCore {
  protected readonly canvas: HTMLCanvasElement;
  protected readonly renderer: THREE.WebGLRenderer;
  /** The size the renderer was last told about, so `paint` can notice a stale buffer. */
  private sizedW = 0;
  private sizedH = 0;
  protected readonly scene = new THREE.Scene();
  protected readonly camera = new THREE.PerspectiveCamera(40, 1, 0.1, 50);

  constructor(canvas: HTMLCanvasElement, opts: PetOptions = {}) {
    super(opts);
    this.canvas = canvas;
    this.renderer = new THREE.WebGLRenderer({
      canvas,
      alpha: true,
      antialias: opts.antialias ?? true,
    });
    this.renderer.outputColorSpace = THREE.LinearSRGBColorSpace;
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, opts.pixelRatioCap ?? 2));
    this.camera.position.z = 5;
  }

  protected paint(dt: number): void {
    // The element's box is the coordinate space, the same rule the 2D base follows: a mount
    // during a layout change (the panel's entrance, a dock resize) can leave the drawing
    // buffer at an older size, and the browser then stretches it to the CSS box — the pet
    // reads as squeezed horizontally. Reconcile here, so every frame draws at its size.
    const w = this.canvas.clientWidth;
    const h = this.canvas.clientHeight;
    if (w >= 1 && h >= 1 && (w !== this.sizedW || h !== this.sizedH)) {
      this.applyResize(w, h);
      this.onResize(w, h);
    }
    this.tick(dt);
    this.renderer.render(this.scene, this.camera);
  }

  protected applyResize(w: number, h: number): void {
    this.sizedW = w;
    this.sizedH = h;
    this.renderer.setSize(w, h, false);
    this.camera.aspect = w / h;
    this.camera.updateProjectionMatrix();
  }

  protected onDispose(): void {
    this.scene.traverse((o) => {
      const m = o as THREE.Mesh;
      m.geometry?.dispose();
      const mats = Array.isArray(m.material) ? m.material : m.material ? [m.material] : [];
      for (const x of mats) {
        (x as THREE.MeshBasicMaterial).map?.dispose();
        x.dispose();
      }
    });
    this.renderer.dispose();
  }

  /** The variant's per-frame update; `paint` renders right after it. */
  protected abstract tick(dt: number): void;
}
