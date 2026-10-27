/**
 * The half every pet shares, whatever paints it: state, listeners, the clock, the energy
 * pulse, the gaze interpolation, the one-shot table and the still/paused paths.
 *
 * Extracted from `PetBase` when the Canvas 2D family arrived: a 2D character cannot extend
 * a class that builds a `WebGLRenderer` in its constructor. This file imports only
 * `./types`, so nothing that reaches it pulls `three`.
 *
 * The split is a contract: a subclass paints (`paint`) and settles its own interpolation
 * (`snap`); everything about *when* to paint is here.
 */
import { PET_STATES, type PetController, type PetOptions, type PetState } from "./types";

export abstract class PetCore implements PetController {
  /** One-shot states: seconds before the automatic return to idle. */
  protected abstract readonly once: Partial<Record<PetState, number>>;

  protected _state: PetState = "idle";
  protected t = 0;
  protected t0 = 0;
  protected energy = 0;
  protected look = { x: 0, y: 0 };
  protected lookC = { x: 0, y: 0 };
  protected readonly opts: PetOptions;

  private listeners = new Set<(s: PetState) => void>();
  private raf = 0;
  private last = 0;
  private running = true;
  private paused = false;
  protected still = false;

  constructor(opts: PetOptions = {}) {
    this.opts = opts;
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
    // The family sizes its surface first, then the variant's hook runs: a variant that
    // overrides the hook must never be able to skip the sizing, which is how a scene ends
    // up drawn for one viewport and stretched into another.
    this.applyResize(w, h);
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
    this.onDispose();
  }

  /** Call at the end of a variant's constructor. */
  protected start(): void {
    this.startLoop();
  }

  protected onEnter(_s: PetState): void {}
  protected onPulse(_amount: number): void {}
  /** Sizes the family's surface for a viewport. Never overridden by a variant. */
  protected abstract applyResize(w: number, h: number): void;
  /** A viewport change, after the surface is sized: a variant invalidates what it must. */
  protected onResize(_w: number, _h: number): void {}
  /** Release whatever the family owns: registers, geometries, textures. */
  protected onDispose(): void {}

  /** Paints one frame: the family's tick and its render. */
  protected abstract paint(dt: number): void;

  /**
   * Puts a variant's interpolated values straight on the current state's preset, for a
   * still frame. Each variant overrides it; the base then runs one zero-delta paint, so the
   * geometry and the uniforms follow the snap.
   */
  protected abstract snap(): void;

  /** The gaze and energy of a still frame are its state's alone: no pulse, no look. */
  private resetTransients(): void {
    this.energy = 0;
    this.look.x = 0;
    this.look.y = 0;
    this.lookC.x = 0;
    this.lookC.y = 0;
  }

  private renderOnce(): void {
    this.paint(0);
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

    // An ending returns to idle before the frame is painted, so the frame a reader sees when
    // a celebration expires is the rest, not one more frame of the celebration.
    const o = this.once[this._state];
    if (o !== undefined && this.t - this.t0 > o) this.setState("idle");

    this.paint(dt);
  };
}
