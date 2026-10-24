/**
 * The pet's base class: one renderer, one scene, one clock.
 *
 * Vendored from `context/refs/react/pet-react/PetBase.ts` (this project's own material),
 * with comments translated and two additions the panel needs: a still mode, so reduced
 * motion and a resting pet get one settled frame instead of a loop, and a disposal path
 * that is safe to call once per mounted strip.
 */
import * as THREE from "three";
import { PET_STATES, type PetController, type PetOptions, type PetState } from "./types";

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

export abstract class PetBase implements PetController {
  protected readonly renderer: THREE.WebGLRenderer;
  protected readonly scene = new THREE.Scene();
  protected readonly camera = new THREE.PerspectiveCamera(40, 1, 0.1, 50);

  /** One-shot states: seconds before the automatic return to idle. */
  protected abstract readonly once: Partial<Record<PetState, number>>;

  protected _state: PetState = "idle";
  protected t = 0;
  protected t0 = 0;
  protected energy = 0;
  protected look = { x: 0, y: 0 };
  protected lookC = { x: 0, y: 0 };

  private listeners = new Set<(s: PetState) => void>();
  private raf = 0;
  private last = 0;
  private running = true;
  private paused = false;
  private still = false;
  protected readonly opts: PetOptions;

  constructor(canvas: HTMLCanvasElement, opts: PetOptions = {}) {
    this.opts = opts;
    this.renderer = new THREE.WebGLRenderer({
      canvas,
      alpha: true,
      antialias: opts.antialias ?? true,
    });
    this.renderer.outputColorSpace = THREE.LinearSRGBColorSpace;
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, opts.pixelRatioCap ?? 2));
    this.camera.position.z = 5;
  }

  get state(): PetState {
    return this._state;
  }

  setState(s: PetState): void {
    if (!PET_STATES.includes(s) || s === this._state) return;
    this._state = s;
    this.t0 = this.t;
    this.onEnter(s);
    if (this.still) {
      // A still pet paints the new state once: the preset is applied immediately, because a
      // transition that no loop advances would leave the previous state's shape on screen.
      this.resetTransients();
      this.snap();
      this.renderOnce();
    }
    this.listeners.forEach((f) => f(s));
  }

  pulse(amount = 0.6): void {
    this.energy = Math.min(1.5, this.energy + amount);
    this.onPulse(amount);
  }

  lookAt(x: number, y: number): void {
    this.look.x = Math.max(-1, Math.min(1, x));
    this.look.y = Math.max(-1, Math.min(1, y));
  }

  on(_event: "stateChange", cb: (s: PetState) => void): () => void {
    this.listeners.add(cb);
    return () => {
      this.listeners.delete(cb);
    };
  }

  resize(w: number, h: number): void {
    if (w < 1 || h < 1) return;
    this.renderer.setSize(w, h, false);
    this.camera.aspect = w / h;
    this.camera.updateProjectionMatrix();
    this.onResize(w, h);
    if (this.still) this.renderOnce();
  }

  setPaused(p: boolean): void {
    if (this.paused === p) return;
    this.paused = p;
    if (!p && this.running && !this.still && !this.raf) this.startLoop();
  }

  /**
   * Switches between the animated loop and still frames. Still is the shape reduced motion
   * and a resting pet need: the loop stops, the interpolation snaps to the target preset
   * and one frame is painted. A paused loop cannot do this — it never settles its own
   * transitions, so its first frame would show the state before the change.
   */
  setStill(on: boolean): void {
    if (this.still === on) return;
    this.still = on;
    if (on) {
      this.stopLoop();
      this.resetTransients();
      this.snap();
      this.renderOnce();
      return;
    }
    if (this.running && !this.paused) this.startLoop();
  }

  dispose(): void {
    this.running = false;
    this.stopLoop();
    this.listeners.clear();
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

  /** Call at the end of a variant's constructor. */
  protected start(): void {
    this.startLoop();
  }

  protected onEnter(_s: PetState): void {}
  protected onPulse(_amount: number): void {}
  protected onResize(_w: number, _h: number): void {}
  protected abstract tick(dt: number): void;

  /**
   * Puts a variant's interpolated values straight on the current state's preset, for a
   * still frame. Each variant overrides it; the base then runs one zero-delta tick, so the
   * geometry and the uniforms follow the snap.
   */
  protected snap(): void {}

  /** The gaze and energy of a still frame are its state's alone: no pulse, no look. */
  private resetTransients(): void {
    this.energy = 0;
    this.look.x = 0;
    this.look.y = 0;
    this.lookC.x = 0;
    this.lookC.y = 0;
  }

  private renderOnce(): void {
    this.tick(0);
    this.renderer.render(this.scene, this.camera);
  }

  private startLoop(): void {
    if (this.raf || !this.running || this.paused || this.still) return;
    this.last = performance.now();
    this.raf = requestAnimationFrame(this.loop);
  }

  private stopLoop(): void {
    if (this.raf) cancelAnimationFrame(this.raf);
    this.raf = 0;
  }

  private loop = (now: number): void => {
    this.raf = 0;
    if (!this.running || this.paused || this.still) return;
    this.raf = requestAnimationFrame(this.loop);

    const dt = Math.min((now - this.last) / 1000, 0.05);
    this.last = now;
    this.t += dt;
    this.energy *= Math.exp(-dt * 4);
    const lk = 1 - Math.exp(-dt * 8);
    this.lookC.x += (this.look.x - this.lookC.x) * lk;
    this.lookC.y += (this.look.y - this.lookC.y) * lk;

    this.tick(dt);

    const o = this.once[this.state];
    if (o !== undefined && this.t - this.t0 > o) this.setState("idle");
    this.renderer.render(this.scene, this.camera);
  };
}
