/**
 * Scriptor — a cloud of letters. It has no body: it has an alphabet, and every state is a
 * word the cloud learns to write. In "sphere" the letters are a creature in motion; in
 * "line" they are a sentence — the same letters, laid on a line of text. Each state's
 * character is its word, not an accessory colour.
 *
 * Vendored from `context/refs/react/pet/proposals/Scriptor, pet di parole.html` (this
 * project's own material), translated and reduced to the panel's contract: no toolbar, no
 * scripted demo, no deep link and no `hold`.
 */
import { Pet2D } from "../Pet2D";
import type { PetOptions, PetState, PetTheme } from "../../pet/types";

interface Preset {
  word: string;
  form: "sphere" | "line" | "scatter";
  c: string;
  spin: number;
  churn: number;
  tight: number;
  sizeK: number;
  wave: number;
  drift: number;
  bob: number;
  bobS: number;
  alpha: number;
  rate: number;
  once: number;
  cyc?: number;
  emit?: number;
}

// The state colour is the character's identity; the theme decides whether the ink is
// darkened (light paper) or lightened (dark background). No semantic colour.
const STATES: Record<PetState, Preset> = {
  idle: {
    word: "idle",
    form: "sphere",
    c: "#7b6cff",
    spin: 0.22,
    churn: 0.5,
    tight: 1,
    sizeK: 1,
    wave: 0,
    drift: 0.1,
    bob: 0.05,
    bobS: 1.2,
    alpha: 0.62,
    rate: 3,
    once: 0,
  },
  listening: {
    word: "listen",
    form: "sphere",
    c: "#2b93cc",
    spin: 0.08,
    churn: 0.2,
    tight: 0.88,
    sizeK: 1.4,
    wave: 0,
    drift: 0.05,
    bob: 0.03,
    bobS: 1.6,
    alpha: 0.8,
    rate: 3.5,
    once: 0,
  },
  thinking: {
    word: "think",
    form: "sphere",
    c: "#9a5cff",
    spin: 1.0,
    churn: 7,
    tight: 0.94,
    sizeK: 1,
    wave: 0,
    drift: 0.22,
    bob: 0.04,
    bobS: 2.2,
    alpha: 0.72,
    rate: 4,
    once: 0,
  },
  working: {
    word: "run",
    form: "line",
    c: "#c47c1c",
    spin: 0.12,
    churn: 0.3,
    tight: 1,
    sizeK: 1,
    wave: 0,
    drift: 0.04,
    bob: 0,
    bobS: 3.2,
    alpha: 0.95,
    rate: 6,
    once: 0,
    cyc: 3.4,
    emit: 0,
  },
  speaking: {
    word: "speak",
    form: "line",
    c: "#1f8a55",
    spin: 0.12,
    churn: 0.2,
    tight: 1,
    sizeK: 1,
    wave: 0.6,
    drift: 0.04,
    bob: 0,
    bobS: 2.6,
    alpha: 0.95,
    rate: 5.5,
    once: 0,
    cyc: 0,
    emit: 0.12,
  },
  success: {
    word: "ok",
    form: "line",
    c: "#a8810a",
    spin: 0.35,
    churn: 0.3,
    tight: 1,
    sizeK: 1,
    wave: 0.8,
    drift: 0.08,
    bob: 0.05,
    bobS: 2,
    alpha: 1,
    rate: 7,
    once: 2.6,
  },
  error: {
    word: "fail",
    form: "scatter",
    c: "#c0394c",
    spin: 0.08,
    churn: 9,
    tight: 1,
    sizeK: 1,
    wave: 0,
    drift: 0.42,
    bob: 0.02,
    bobS: 2,
    alpha: 0.85,
    rate: 5,
    once: 2.0,
  },
  sleeping: {
    word: "sleep",
    form: "line",
    c: "#4d5480",
    spin: 0.03,
    churn: 0.08,
    tight: 1,
    sizeK: 0.9,
    wave: 0.05,
    drift: 0.02,
    bob: 0.02,
    bobS: 0.5,
    alpha: 0.32,
    rate: 2.2,
    once: 0,
  },
};

const NUMBERS = [
  "spin",
  "churn",
  "tight",
  "sizeK",
  "wave",
  "drift",
  "bob",
  "bobS",
  "alpha",
  "rate",
] as const;

const FONT = 'ui-monospace,"SF Mono",Menlo,Consolas,monospace';
const GLYPHS = "·:;.,'\"-_/\\|()[]{}<>*+=?!0123456789";

const hex = (h: string): number[] => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16));
const mix = (a: number[], b: number[], t: number): number[] =>
  a.map((v, i) => Math.round(v + (b[i] - v) * t));
const rgb = (v: number[]): string => `rgb(${v[0]},${v[1]},${v[2]})`;

interface Glyph {
  dx: number;
  dy: number;
  dz: number;
  rr: number;
  seed: number;
  x: number;
  y: number;
  z: number;
  flip: number;
  ch: string;
  pz: number;
  glyph?: string;
  depth?: number;
  size?: number;
}

interface Escape {
  x: number;
  y: number;
  vx: number;
  vy: number;
  life: number;
  ch: string;
  seed: number;
}

interface LetterPoints {
  letters: number[][][];
  adv: number;
}

export class ScriptorPet extends Pet2D {
  protected readonly once: Partial<Record<PetState, number>> = {
    success: STATES.success.once,
    error: STATES.error.once,
  };

  private readonly glyphs: Glyph[];
  /** The glyph count: the visible effect of `density`. */
  readonly budget: number;
  private cur: Record<string, number> = {};
  private ink = "";
  private ang = 0;
  private burst = 0;
  private escape: Escape[] = [];
  private lastEmit = 0;
  private cacheKey = "";
  private cache: LetterPoints | null = null;

  constructor(canvas: HTMLCanvasElement, opts: PetOptions = {}, theme: PetTheme = "dark") {
    super(canvas, opts, theme);
    const density = Math.min(1.2, Math.max(0.35, opts.density ?? 1));
    const n = (this.budget = Math.round(360 * density));
    this.glyphs = [];
    for (let i = 0; i < n; i++) {
      const z = 1 - ((i + 0.5) / n) * 2;
      const r = Math.sqrt(1 - z * z);
      const th = i * 2.399963;
      this.glyphs.push({
        dx: Math.cos(th) * r,
        dy: z,
        dz: Math.sin(th) * r,
        rr: 0.84 + Math.random() * 0.2,
        seed: Math.random(),
        x: 0,
        y: 0,
        z: 0,
        flip: 0,
        ch: "",
        pz: 0,
      });
    }
    for (const key of NUMBERS) this.cur[key] = STATES.idle[key];
    this.deriveInk();
    // The loop is the pet: without it a 2D character paints only when a still frame or a
    // state change asks, which leaves an animated idle blank.
    this.start();
  }

  protected onTheme(): void {
    this.deriveInk();
  }

  protected onEnter(s: PetState): void {
    this.energy = 0;
    if (s === "success") this.burst = 1;
    this.deriveInk();
  }

  protected onPulse(): void {
    // A token leaving the word: the push starts on the right.
    for (let i = 0; i < 3; i++) this.spawn();
  }

  /** A still frame sits exactly on the state's preset and clears the in-flight particles. */
  protected snap(): void {
    const preset = STATES[this._state];
    for (const key of NUMBERS) this.cur[key] = preset[key];
    this.burst = 0;
    this.escape = [];
  }

  private deriveInk(): void {
    const c = hex(STATES[this._state].c);
    this.ink = rgb(
      this.theme === "dark" ? mix(c, [255, 255, 255], 0.18) : mix(c, [14, 16, 24], 0.42),
    );
  }

  private spawn(): void {
    if (this.escape.length > 90) this.escape.shift();
    this.escape.push({
      x: this.w / 2 + (Math.random() - 0.15) * this.w * 0.1,
      y: this.h / 2 + (Math.random() - 0.5) * this.h * 0.16,
      vx: 26 + Math.random() * 46,
      vy: -16 - Math.random() * 30,
      life: 1,
      ch: GLYPHS[(Math.random() * GLYPHS.length) | 0],
      seed: Math.random(),
    });
  }

  /** The advance of one character at a size; the font is monospaced. */
  private advance(size: number): number {
    this.ctx.font = `500 ${size}px ${FONT}`;
    const m = this.ctx.measureText?.("M");
    return m && typeof m.width === "number" && m.width > 0 ? m.width : size * 0.6;
  }

  /**
   * The inked points of each letter, at the line's size: the word's letters are not a
   * rectangle to fill, they are a shape to cover. Every glyph lands on a point of its own
   * letter, so the word really reads.
   */
  private letterPoints(word: string, size: number): LetterPoints {
    const key = `${word}|${Math.round(size)}`;
    if (this.cacheKey === key && this.cache) return this.cache;
    const empty = (): LetterPoints => ({
      letters: [...word].map(() => []),
      adv: Math.max(1, size * 0.6),
    });
    // Before the first resize there is no size to sample with, and jsdom has no context.
    if (size < 6 || typeof document === "undefined") return empty();
    const S = Math.ceil(size * 1.6);
    const cv = document.createElement("canvas");
    cv.width = cv.height = S;
    const cx = cv.getContext("2d", { willReadFrequently: true });
    if (!cx) return empty();
    const adv = this.advance(size);
    const letters: number[][][] = [];
    for (const ch of word) {
      cx.clearRect(0, 0, S, S);
      cx.font = `500 ${size}px ${FONT}`;
      cx.textAlign = "center";
      cx.textBaseline = "middle";
      cx.fillStyle = "#000";
      cx.fillText(ch, S / 2, S / 2);
      const d = cx.getImageData(0, 0, S, S).data;
      const pts: number[][] = [];
      const step = Math.max(2, Math.round(size / 20));
      for (let y = 0; y < S; y += step) {
        for (let x = 0; x < S; x += step) {
          if (d[(y * S + x) * 4 + 3] > 128) pts.push([x - S / 2 + adv / 2, y - S / 2]);
        }
      }
      // Shuffle: two glyphs must not stack on neighbouring points.
      for (let i = pts.length - 1; i > 0; i--) {
        const j = (Math.random() * (i + 1)) | 0;
        const t = pts[i];
        pts[i] = pts[j];
        pts[j] = t;
      }
      letters.push(pts);
    }
    const value = { letters, adv };
    this.cacheKey = key;
    this.cache = value;
    return value;
  }

  protected tick(dt: number): void {
    const preset = STATES[this._state];
    const k = 1 - Math.exp(-dt * 5);
    const c = this.cur;
    for (const key of NUMBERS) c[key] += (preset[key] - c[key]) * k;
    this.burst *= Math.exp(-dt * 2.2);

    // Whoever speaks emits on its own, even without a pulse: the word is never still.
    if (preset.emit && this.t - this.lastEmit > preset.emit) {
      this.lastEmit = this.t;
      this.spawn();
    }
    for (let i = this.escape.length - 1; i >= 0; i--) {
      const e = this.escape[i];
      e.x += e.vx * dt;
      e.y += e.vy * dt;
      e.vy -= 10 * dt;
      e.life -= dt * 0.75;
      if (e.life <= 0 || e.x > this.w + 20) this.escape.splice(i, 1);
    }
    this.ang += dt * c.spin;

    this.draw(dt, preset);
  }

  private draw(dt: number, P: Preset): void {
    const ctx = this.ctx;
    const w = this.w;
    const h = this.h;
    const c = this.cur;

    const cx = w / 2 + this.lookC.x * w * 0.03;
    const cy = h / 2 - this.lookC.y * h * 0.06 - Math.sin(this.t * c.bobS) * c.bob * h;
    const R = Math.min(h * 0.4, w * 0.15) * c.tight;

    // The line of text: the size chosen to fit the band.
    const lineSize = Math.min(h * 0.52, w * 0.115) * c.sizeK;
    const word = P.word;
    const n = word.length;
    const lp = this.letterPoints(word, lineSize);
    const adv = lp.adv;
    const total = adv * n;
    const x0 = -total / 2;

    // In "working" the word writes itself over again: the sentence is a gesture, not an
    // image. .45 writes it, .6 holds it, then it dissolves from the right.
    // A still frame shows the sentence already written: a reduced-motion reader gets the
    // word, not the cloud it forms from.
    let reveal = 1;
    if (P.form === "line" && !this.still) {
      if (P.cyc) {
        const q = ((this.t - this.t0) % P.cyc) / P.cyc;
        const level = q < 0.45 ? q / 0.45 : q < 0.66 ? 1 : Math.max(0, 1 - (q - 0.66) / 0.34);
        reveal = 0.34 + 0.66 * level; // the word never disappears completely
      } else {
        reveal = Math.min(1, (this.t - this.t0) / 1.6);
      }
    }
    ctx.font = `500 ${lineSize}px ${FONT}`;
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";

    const order: Glyph[] = [];
    for (let i = 0; i < this.budget; i++) {
      const g = this.glyphs[i];
      const li = i % n;
      const pt = (i / n) | 0;

      // --- target ---
      let tx: number;
      let ty: number;
      let tz: number;
      const onLine = P.form !== "sphere" && (li + 1) / n <= reveal + 0.001;
      if (P.form === "sphere" || !onLine) {
        const a = this.ang + g.seed * 0.6;
        const ca = Math.cos(a);
        const sa = Math.sin(a);
        const x = g.dx * ca - g.dz * sa;
        const z = g.dx * sa + g.dz * ca;
        const r = R * g.rr * (1 + Math.sin(this.t * 1.3 + g.seed * 6.28) * 0.03);
        tx = x * r;
        ty = g.dy * r;
        tz = z;
      } else {
        const pts = lp.letters[li] || [];
        const p = pts.length ? pts[pt % pts.length] : null;
        if (p) {
          const swayL = P.emit ? Math.sin(this.t * 5 + li * 1.15) * lineSize * 0.05 : 0;
          tx = x0 + adv * li + adv / 2 + p[0] + (g.seed * 2 - 1) * 1.4 + swayL;
          ty =
            p[1] +
            (Math.sin(g.seed * 37.7) * 2 - 1) * 1.4 +
            (P.emit ? Math.cos(this.t * 4.3 + li) * lineSize * 0.03 : 0);
          tz = Math.sin(g.seed * 11.3) * 0.12;
        } else {
          const a2 = this.ang + g.seed * 0.6;
          const ca2 = Math.cos(a2);
          const sa2 = Math.sin(a2);
          const xx = g.dx * ca2 - g.dz * sa2;
          const zz = g.dx * sa2 + g.dz * ca2;
          tx = xx * R * g.rr;
          ty = g.dy * R * g.rr;
          tz = zz;
        }
      }

      // --- offset (error, the success burst, the energy current) ---
      if (P.form === "scatter") {
        tx += g.dx * (1 + g.seed) * c.drift * R * 1.2;
        ty += g.dy * (1 + g.seed) * c.drift * R * 1.2;
      }
      tx += g.dx * this.burst * R * 0.9;
      ty += g.dy * this.burst * R * 0.9;
      tx += Math.sin(this.t * 7 + g.seed * 40) * this.energy * 6;
      ty += Math.cos(this.t * 6 + g.seed * 30) * this.energy * 6;

      // A still frame jumps straight to the targets: a word that only exists after two
      // seconds of interpolation would never be written for a reduced-motion reader.
      const kk = this.still ? 1 : 1 - Math.exp(-dt * c.rate);
      g.x += (tx - g.x) * kk;
      g.y += (ty - g.y) * kk;
      g.z += (tz - g.z) * kk;

      // --- character: its own letter, and every now and then a noise glyph ---
      if (g.flip > 0) g.flip -= dt;
      else if (Math.random() < P.churn * dt * 0.9) {
        g.flip = 0.12 + Math.random() * 0.22;
        g.ch = GLYPHS[(Math.random() * GLYPHS.length) | 0];
      }
      const glyph = g.flip > 0 && P.churn > 1 ? g.ch : word[li];
      g.pz = g.z;
      g.glyph = glyph;
      g.depth = g.z * 0.5 + 0.5;
      g.size =
        lineSize * (onLine ? 0.105 : 0.21) * (0.82 + g.seed * 0.42) * (1 + this.energy * 0.12);
      order.push(g);
    }

    order.sort((a, b) => a.pz - b.pz);

    ctx.fillStyle = this.ink;
    for (const g of order) {
      const persp = P.form === "sphere" ? 1 + g.pz * 0.3 : 1;
      const wave = 1 + c.wave * 0.18 * Math.sin(g.x * 0.06 - this.t * 7 + g.seed * 3);
      const a =
        c.alpha *
        (P.form === "sphere" ? 0.16 + 0.84 * (g.depth ?? 0) ** 2 : 0.8 + 0.2 * (g.depth ?? 0));
      ctx.globalAlpha = Math.max(0, Math.min(1, a));
      const s = (g.size ?? 0) * persp * wave;
      ctx.font = `500 ${s.toFixed(1)}px ${FONT}`;
      ctx.fillText(g.glyph ?? "", cx + g.x, cy + g.y);
    }
    ctx.globalAlpha = 1;

    // The caret: where the word is being written now. It never disappears, or the state
    // would read as a figure instead of an action in progress.
    if (P.form === "line" && (P.cyc || reveal < 0.999)) {
      const caretX = cx + x0 + adv * n * reveal;
      const on = P.cyc ? true : Math.sin(this.t * 9) > -0.35;
      if (on) {
        const hh = lineSize * (0.42 + 0.06 * Math.sin(this.t * 7));
        ctx.fillStyle = this.ink;
        ctx.globalAlpha = 0.75;
        ctx.fillRect(caretX - 1.5, cy - hh / 2, 3, hh);
        ctx.globalAlpha = 0.18;
        ctx.fillRect(caretX - 4, cy - hh / 2 - 2, 8, hh + 4);
        ctx.globalAlpha = 1;
      }
    }

    // The particles: the word sheds pieces to the right.
    for (const e of this.escape) {
      ctx.globalAlpha = Math.max(0, e.life) * 0.9;
      ctx.fillStyle = this.ink;
      const sz = lineSize * (0.1 + e.seed * 0.06);
      ctx.font = `500 ${sz.toFixed(1)}px ${FONT}`;
      ctx.fillText(e.ch, e.x, e.y);
    }
    ctx.globalAlpha = 1;
  }
}
