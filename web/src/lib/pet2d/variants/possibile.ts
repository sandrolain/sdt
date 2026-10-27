/**
 * Possibile — a self-portrait that scrolls: the present stands still on the right, and the
 * sentence I have already said flows under it from the left. The fan is what I have not
 * said yet.
 *
 * Vendored from `context/refs/react/pet/proposals/Possibile, come mi vedo.html` (this
 * project's own material), translated and reduced to the panel's contract: no toolbar, no
 * scripted demo, no deep link and no `hold`.
 */
import { Pet2D } from "../Pet2D";
import type { PetOptions, PetState, PetTheme } from "../../pet/types";

interface Preset {
  c: string;
  speed: number;
  spread: number;
  fan: number;
  trail: number;
  alpha: number;
  flat?: number;
  ticks: number;
  once: number;
}

// The state colour is the character's identity: raw values, like the other pets' palettes.
const STATES: Record<PetState, Preset> = {
  idle: {
    c: "#6f7bd6",
    speed: 0.55,
    spread: 0.36,
    fan: 0.55,
    trail: 0.75,
    alpha: 1,
    flat: 0,
    ticks: 0,
    once: 0,
  },
  listening: {
    c: "#3f9ec9",
    speed: 0.22,
    spread: 0.16,
    fan: 0.7,
    trail: 0.85,
    alpha: 1,
    flat: 0,
    ticks: 0,
    once: 0,
  },
  thinking: {
    c: "#a477ea",
    speed: 0.85,
    spread: 1.25,
    fan: 1,
    trail: 0.7,
    alpha: 0.95,
    ticks: 0,
    once: 0,
  },
  working: {
    c: "#c98a3a",
    speed: 2.6,
    spread: 0.72,
    fan: 0.8,
    trail: 1,
    alpha: 1,
    flat: 0,
    ticks: 0,
    once: 0,
  },
  speaking: {
    c: "#3f9e6e",
    speed: 1.85,
    spread: 0.3,
    fan: 0.4,
    trail: 1,
    alpha: 1,
    flat: 0,
    ticks: 1,
    once: 0,
  },
  success: {
    c: "#b28a20",
    speed: 1.0,
    spread: 0,
    fan: 0.55,
    trail: 1,
    alpha: 1,
    flat: 0,
    ticks: 0.4,
    once: 2.4,
  },
  error: {
    c: "#cc4a4a",
    speed: 0.1,
    spread: 0.95,
    fan: 0.5,
    trail: 0.8,
    alpha: 1,
    flat: 0,
    ticks: 0,
    once: 1.8,
  },
  sleeping: {
    c: "#5b6382",
    speed: 0.05,
    spread: 0.05,
    fan: 0.12,
    trail: 0.35,
    alpha: 0.7,
    flat: 1,
    ticks: 0,
    once: 0,
  },
};

const NUMBERS = ["speed", "spread", "fan", "trail", "alpha", "ticks"] as const;

const hex = (h: string): number[] => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16));
const mix = (a: number[], b: number[], t: number): number[] =>
  a.map((v, i) => Math.round(v + (b[i] - v) * t));
const rgb = (v: number[]): string => `rgb(${v[0]},${v[1]},${v[2]})`;

interface Tick {
  X: number;
  life: number;
  seed: number;
}

interface Harmonic {
  k: number;
  a: number;
  p: number;
  d: number;
}

export class PossibilePet extends Pet2D {
  protected readonly once: Partial<Record<PetState, number>> = {
    success: STATES.success.once,
    error: STATES.error.once,
  };

  private cur: Record<string, number> = {};
  private ink = "";
  private X = 0;
  private flash = 0;
  private gapX = -1e9;
  private ticks: Tick[] = [];
  /** The chosen line's character: three harmonics that drift slowly. */
  private readonly wave: Harmonic[];

  constructor(canvas: HTMLCanvasElement, opts: PetOptions = {}, theme: PetTheme = "dark") {
    super(canvas, opts, theme);
    for (const key of NUMBERS) this.cur[key] = STATES.idle[key];
    this.wave = [0, 1, 2].map((i) => ({
      k: 0.9 + i * 0.9 + Math.random() * 0.5,
      a: 0.05 + i * 0.03,
      p: Math.random() * 6.28,
      d: 0.1 + i * 0.06,
    }));
    this.deriveInk();
    // The loop is the pet: without it a 2D character paints only when a still frame or a
    // state change asks, which leaves an animated idle blank.
    this.start();
  }

  protected onTheme(): void {
    this.deriveInk();
  }

  protected onEnter(s: PetState): void {
    if (s === "success") this.flash = 1;
    if (s === "error") {
      this.gapX = this.X;
      this.flash = 1;
    }
    if (s === "idle" || s === "sleeping") this.gapX = -1e9;
    this.deriveInk();
  }

  protected onPulse(): void {
    this.ticks.push({ X: this.X, life: 1, seed: Math.random() });
    if (this.ticks.length > 140) this.ticks.shift();
  }

  /** A still frame sits exactly on the state's preset and forgets the in-flight marks. */
  protected snap(): void {
    const preset = STATES[this._state];
    for (const key of NUMBERS) this.cur[key] = preset[key];
    this.flash = 0;
    this.ticks = [];
    if (this._state === "error") this.gapX = this.X;
  }

  private deriveInk(): void {
    const c = hex(STATES[this._state].c);
    this.ink = rgb(
      this.theme === "dark" ? mix(c, [240, 242, 255], 0.28) : mix(c, [18, 22, 30], 0.42),
    );
  }

  // The present does not move: it stands on the right and the text scrolls under it from
  // the left. X is the content coordinate (always growing); the screen is
  // x = headX + (X - this.X), never returning to the starting point.
  private headX(): number {
    return this.w * 0.6 + Math.sin(this.t * 0.22) * this.w * 0.012;
  }

  // The chosen line: the sum of three harmonics on the content, not on the screen.
  private sy(X: number): number {
    const u = X / (this.h * 1.7);
    let v = 0;
    for (let i = 0; i < 3; i++) {
      const q = this.wave[i];
      v += q.a * Math.sin(q.k * u + q.p + this.t * q.d);
    }
    return this.h / 2 + (v / 0.28) * this.h * 0.14 - this.lookC.y * this.h * 0.06;
  }

  private sx(X: number): number {
    return this.headX() + (X - this.X);
  }

  protected tick(dt: number): void {
    const preset = STATES[this._state];
    const k = 1 - Math.exp(-dt * 3);
    const c = this.cur;
    for (const key of NUMBERS) c[key] += (preset[key] - c[key]) * k;
    this.flash *= Math.exp(-dt * 2.2);

    // The present advances in the content, not on the screen: the tail leaves on the left.
    this.X += dt * c.speed * 58;
    for (let i = this.ticks.length - 1; i >= 0; i--) {
      const t = this.ticks[i];
      t.life -= dt * (this._state === "speaking" ? 0.12 : 0.35);
      if (t.life <= 0 || this.sx(t.X) < -20) this.ticks.splice(i, 1);
    }

    this.draw(preset, c);
  }

  private draw(_P: Preset, c: Record<string, number>): void {
    const ctx = this.ctx;
    const w = this.w;
    const h = this.h;
    const pad = Math.max(24, w * 0.06);
    const hx = this.headX();
    const hy = this.sy(this.X);

    // ---- The fan: the sentences I have not said yet -------------------------
    const fanA = c.fan;
    if (fanA > 0.02) {
      ctx.strokeStyle = this.ink;
      ctx.lineCap = "round";
      const N = 24;
      for (let i = 0; i < N; i++) {
        const a = (i / (N - 1) - 0.5) * 2;
        const near = 1 - Math.abs(a) * 0.55;
        const reach = 0.42 + 0.58 * near + Math.sin(this.t * 0.6 + i * 2.1) * 0.06;
        const ex = hx + (w - pad - hx) * Math.max(0.15, Math.min(1, reach));
        const ey = hy + a * c.spread * h * 0.62 + Math.sin(this.t * 0.9 + i) * h * 0.02;
        const curl = Math.sin(this.t * 1.1 + i * 2.3) * h * 0.09 * Math.max(0.25, c.spread);
        const mx = hx + (ex - hx) * 0.52;
        const my = hy + (ey - hy) * 0.52 + curl;
        const c1x = hx + (mx - hx) * 0.45;
        const c1y = hy + (my - hy) * 0.25;
        const c2x = mx + (ex - mx) * 0.55;
        const c2y = my + (ey - my) * 0.7 - curl * 0.35;
        ctx.globalAlpha = Math.max(0, Math.min(1, 0.04 + fanA * 0.2 * near));
        ctx.lineWidth = 0.7 + near * 0.7;
        ctx.beginPath();
        ctx.moveTo(hx, hy);
        ctx.bezierCurveTo(c1x, c1y, mx, my, mx, my);
        ctx.bezierCurveTo(c2x, c2y, ex, ey, ex, ey);
        ctx.stroke();
      }
      // The likeliest one, turning slowly.
      const pickI = Math.floor(this.t * 0.45) % N;
      const pa = (pickI / (N - 1) - 0.5) * 2;
      const pey = hy + pa * c.spread * h * 0.62;
      ctx.globalAlpha = Math.min(1, 0.16 + fanA * 0.42);
      ctx.lineWidth = 1.3;
      ctx.beginPath();
      ctx.moveTo(hx, hy);
      ctx.bezierCurveTo(
        hx + (w - pad - hx) * 0.3,
        hy + (pey - hy) * 0.1,
        hx + (w - pad - hx) * 0.72,
        hy + (pey - hy) * 0.82,
        w - pad,
        pey,
      );
      ctx.stroke();
      ctx.globalAlpha = 1;
    }

    // ---- The line: what I have already said, scrolling away ----------------
    const trailA = c.trail;
    const path: [number, number][] = [];
    const step = 3;
    for (let x = -pad; x <= hx; x += step) {
      const X = this.X - (hx - x);
      path.push([x, this.sy(X)]);
    }
    ctx.lineJoin = "round";
    ctx.lineCap = "round";
    ctx.strokeStyle = this.ink;

    const seg = (from: number, to: number): void => {
      ctx.beginPath();
      for (let i = from; i <= to; i++) {
        if (i === from) ctx.moveTo(path[i][0], path[i][1]);
        else ctx.lineTo(path[i][0], path[i][1]);
      }
      ctx.stroke();
    };
    // Halo.
    ctx.globalAlpha = 0.1 * trailA;
    ctx.lineWidth = 7;
    seg(0, path.length - 1);
    ctx.globalAlpha = trailA;
    // The sentence may have broken: the break scrolls with the text.
    const gapSX = this.sx(this.gapX);
    if (gapSX > -20 && gapSX < hx) {
      const iGap = path.findIndex((p) => p[0] >= gapSX);
      if (iGap > 2 && iGap < path.length - 2) {
        ctx.lineWidth = 2;
        seg(0, iGap - 3);
        seg(iGap + 3, path.length - 1);
        ctx.globalAlpha = trailA * 0.8;
        ctx.lineWidth = 1.4;
        const gx = path[iGap][0];
        const gy = path[iGap][1];
        ctx.beginPath();
        ctx.moveTo(gx, gy - 8);
        ctx.lineTo(gx + 7, gy + 9);
        ctx.stroke();
        ctx.beginPath();
        ctx.moveTo(gx, gy + 8);
        ctx.lineTo(gx - 6, gy - 10);
        ctx.stroke();
      } else {
        ctx.lineWidth = 2;
        seg(0, path.length - 1);
      }
    } else {
      ctx.lineWidth = 2;
      seg(0, path.length - 1);
    }
    ctx.globalAlpha = 1;

    // The token marks: ticks that scroll with the sentence.
    for (const tk of this.ticks) {
      const x = this.sx(tk.X);
      if (x < -10 || x > w) continue;
      ctx.globalAlpha = Math.max(0, tk.life) * 0.55 * trailA;
      ctx.fillStyle = this.ink;
      ctx.fillRect(x, this.sy(tk.X) - 5 - (1 - tk.life) * 7, 1.2, 3 + tk.seed * 3);
    }
    ctx.globalAlpha = 1;

    // ---- The present --------------------------------------------------------
    const pulse = 1 + this.energy * 0.9 + this.flash * 0.9;
    ctx.fillStyle = this.ink;
    ctx.globalAlpha = 0.12;
    ctx.beginPath();
    ctx.arc(hx, hy, 9 * pulse, 0, 6.2832);
    ctx.fill();
    ctx.globalAlpha = 0.28;
    ctx.beginPath();
    ctx.arc(hx, hy, 4.5 * pulse, 0, 6.2832);
    ctx.fill();
    ctx.globalAlpha = 1;
    ctx.beginPath();
    ctx.arc(hx, hy, 2.1 * pulse, 0, 6.2832);
    ctx.fill();

    ctx.globalAlpha = 0.5;
    ctx.fillRect(hx, hy - h * 0.09, 1, h * 0.18);
    ctx.globalAlpha = 1;
  }
}
