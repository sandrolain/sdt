/**
 * The Canvas 2D family's base: a 2D context and device-pixel sizing on the shared clock.
 *
 * The prototypes own a `requestAnimationFrame` loop and a `dpr` cap each; this class is the
 * seam that lets them extend the same `PetCore` the WebGL variants run on, so there is one
 * state machine, one still/paused path and one `once` rule for all six pets.
 *
 * Coordinates a subclass draws in are CSS pixels: the context is scaled by the device pixel
 * ratio before `tick`, and cleared at the backing-store size before it.
 */
import { PetCore } from "../pet/core";
import type { PetOptions, PetTheme } from "../pet/types";

export abstract class Pet2D extends PetCore {
  protected readonly canvas: HTMLCanvasElement;
  protected readonly ctx: CanvasRenderingContext2D;
  protected dpr: number;
  protected w = 0;
  protected h = 0;
  protected theme: PetTheme;

  constructor(canvas: HTMLCanvasElement, opts: PetOptions = {}, theme: PetTheme = "dark") {
    super(opts);
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("a 2D context is unavailable");
    this.canvas = canvas;
    this.ctx = ctx;
    this.theme = theme;
    this.dpr = Math.min(window.devicePixelRatio || 1, opts.pixelRatioCap ?? 2);
  }

  /** The panel's theme reaches the character: ink is derived again, the clock is not reset. */
  setTheme(theme: PetTheme): void {
    if (theme === this.theme) return;
    this.theme = theme;
    this.onTheme();
    if (this.still) this.paint(0);
  }

  protected paint(_dt: number): void {
    // The element's box is the coordinate space: a mount during a layout change (the panel's
    // entrance, a dock resize) can leave the stored size stale, and a stale size is exactly
    // the "stretched" look — the backing store and the CSS box disagree. Reconcile here, so
    // every frame draws in the box it is in.
    const cssW = this.canvas.clientWidth;
    const cssH = this.canvas.clientHeight;
    if (cssW >= 1 && cssH >= 1 && (cssW !== this.w || cssH !== this.h)) this.onResize(cssW, cssH);

    const { ctx, canvas } = this;
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    this.tick(_dt);
  }

  protected onResize(w: number, h: number): void {
    this.w = w;
    this.h = h;
    this.canvas.width = Math.round(w * this.dpr);
    this.canvas.height = Math.round(h * this.dpr);
    this.onSize();
  }

  /** Draws one frame in CSS pixels; the context is already cleared and scaled. */
  protected abstract tick(dt: number): void;

  /** Derives the ink for the new theme. Called at construction time once by the subclass. */
  protected abstract onTheme(): void;

  /** A resize changed `w`/`h`: invalidate any size-derived cache. */
  protected onSize(): void {}
}
