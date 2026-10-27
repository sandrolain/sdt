/**
 * Fiamma — a flame of code: the film's rain, not as a backdrop but as a body.
 *
 * Vendored from `context/refs/react/pet/proposals/Fiamma, pet in stile Matrix.html` (this
 * project's own material), translated and reduced to the panel's contract: no toolbar, no
 * scripted demo, no deep link and no `hold`. The shape is the outline the rain falls in,
 * and two holes are the eyes: the pet has no drawn face, only two gaps in the rain. The
 * state changes the fire — direction, density, width, and how much the flame sways.
 *
 * `eye*` is the expression: opening, tilt (the outer angle), height in space, rim light,
 * how far the gaze wanders, and the shape of the void.
 */
import { Pet2D } from "../Pet2D";
import type { PetOptions, PetState, PetTheme } from "../../pet/types";

interface Preset {
  c: string;
  speed: number;
  w: number;
  h: number;
  lean: number;
  alpha: number;
  dir: number;
  jit: number;
  sweep: number;
  once: number;
  eyeW: number;
  eyeH: number;
  eyeTilt: number;
  eyeDy: number;
  eyeRim: number;
  eyeGlint: number;
  eyeGaze: number;
  eyeCrack: number;
}

// The character's palette: a categorical identity, like a graph cluster's colour, so these
// hex values stay raw instead of becoming semantic tokens.
const STATES: Record<PetState, Preset> = {
  idle: {
    c: "#2f8f5b",
    speed: 1.2,
    w: 1.0,
    h: 1.0,
    lean: 0.35,
    alpha: 0.9,
    dir: 1,
    jit: 0,
    sweep: 0,
    once: 0,
    eyeW: 1.0,
    eyeH: 1.0,
    eyeTilt: 0,
    eyeDy: 0,
    eyeRim: 0.3,
    eyeGlint: 0.55,
    eyeGaze: 0,
    eyeCrack: 0,
  },
  listening: {
    c: "#2f9f86",
    speed: 0.55,
    w: 0.94,
    h: 0.98,
    lean: 0.15,
    alpha: 1,
    dir: 1,
    jit: 0,
    sweep: 0,
    once: 0,
    eyeW: 1.05,
    eyeH: 1.08,
    eyeTilt: -0.03,
    eyeDy: -0.02,
    eyeRim: 0.52,
    eyeGlint: 0.85,
    eyeGaze: 0.22,
    eyeCrack: 0,
  },
  thinking: {
    c: "#3fd07a",
    speed: 1.7,
    w: 0.92,
    h: 1.04,
    lean: 0.85,
    alpha: 1,
    dir: -1,
    jit: 0.15,
    sweep: 0,
    once: 0,
    eyeW: 0.99,
    eyeH: 0.8,
    eyeTilt: 0.07,
    eyeDy: -0.04,
    eyeRim: 0.26,
    eyeGlint: 0.3,
    eyeGaze: 1,
    eyeCrack: 0,
  },
  working: {
    c: "#48e07f",
    speed: 3.8,
    w: 0.7,
    h: 1.1,
    lean: 0.2,
    alpha: 1,
    dir: 1,
    jit: 0,
    sweep: 0,
    once: 0,
    eyeW: 0.97,
    eyeH: 0.72,
    eyeTilt: 0.1,
    eyeDy: 0.01,
    eyeRim: 0.38,
    eyeGlint: 0.42,
    eyeGaze: 0.12,
    eyeCrack: 0,
  },
  speaking: {
    c: "#9dfabf",
    speed: 2.4,
    w: 0.86,
    h: 1.02,
    lean: 0.45,
    alpha: 1,
    dir: 1,
    jit: 0,
    sweep: 0,
    once: 0,
    eyeW: 1.0,
    eyeH: 0.98,
    eyeTilt: -0.02,
    eyeDy: -0.01,
    eyeRim: 0.45,
    eyeGlint: 0.6,
    eyeGaze: 0.08,
    eyeCrack: 0,
  },
  success: {
    c: "#e2ffee",
    speed: 3.0,
    w: 1.02,
    h: 1.12,
    lean: 0.3,
    alpha: 1,
    dir: 1,
    jit: 0,
    sweep: 1,
    once: 2.4,
    eyeW: 1.05,
    eyeH: 0.94,
    eyeTilt: -0.05,
    eyeDy: -0.03,
    eyeRim: 0.68,
    eyeGlint: 1,
    eyeGaze: 0,
    eyeCrack: 0,
  },
  error: {
    c: "#e5564e",
    speed: 1.4,
    w: 0.96,
    h: 0.94,
    lean: 1.1,
    alpha: 1,
    dir: 1,
    jit: 1,
    sweep: 0,
    once: 1.8,
    eyeW: 1.0,
    eyeH: 0.66,
    eyeTilt: 0.18,
    eyeDy: 0.03,
    eyeRim: 0.22,
    eyeGlint: 0.12,
    eyeGaze: 0,
    eyeCrack: 1,
  },
  sleeping: {
    c: "#24523f",
    speed: 0.12,
    w: 0.66,
    h: 0.6,
    lean: 0.05,
    alpha: 0.5,
    dir: 1,
    jit: 0,
    sweep: 0,
    once: 0,
    eyeW: 1.0,
    eyeH: 0.46,
    eyeTilt: 0,
    eyeDy: 0.06,
    eyeRim: 0.14,
    eyeGlint: 0.08,
    eyeGaze: 0,
    eyeCrack: 0,
  },
};

const NUMBERS = [
  "speed",
  "w",
  "h",
  "lean",
  "alpha",
  "jit",
  "sweep",
  "eyeW",
  "eyeH",
  "eyeTilt",
  "eyeDy",
  "eyeRim",
  "eyeGlint",
  "eyeGaze",
  "eyeCrack",
] as const;

// Half a katakana alphabet plus terminal symbols: the code is not read, it is watched.
const GLYPHS = "ｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄﾅﾆﾇﾈﾉﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜﾝ0123456789:.<>=+*";
const FONT = 'ui-monospace,"SF Mono",Menlo,Consolas,monospace';
const TAU = Math.PI * 2;

const hex = (h: string): number[] => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16));
const mix = (a: number[], b: number[], t: number): number[] =>
  a.map((v, i) => Math.round(v + (b[i] - v) * t));
const rgb = (v: number[]): string => `rgb(${v[0]},${v[1]},${v[2]})`;

interface Column {
  x: number;
  y: number;
  speed: number;
  len: number;
  g: string[];
  seed: number;
}

interface Escape {
  x: number;
  y: number;
  vy: number;
  vx: number;
  life: number;
  ch: string;
  seed: number;
}

export class FiammaPet extends Pet2D {
  protected readonly once: Partial<Record<PetState, number>> = {
    success: STATES.success.once,
    error: STATES.error.once,
  };

  private cur: Record<string, number> = {};
  private flash = 0;
  private sweepX = -1;
  private escape: Escape[] = [];
  private tri = hex(STATES.idle.c);
  private ink = "";
  private hot = "";
  private cols: Column[] = [];
  private rows = 1;
  /** The glyph cell in CSS pixels: the visible effect of `density`. */
  readonly cell: number;
  private readonly step: number;

  constructor(canvas: HTMLCanvasElement, opts: PetOptions = {}, theme: PetTheme = "dark") {
    super(canvas, opts, theme);
    const density = Math.min(1.4, Math.max(0.35, opts.density ?? 1));
    this.cell = Math.round(12 / density);
    this.step = Math.max(3, this.cell / 2);
    for (const key of NUMBERS) this.cur[key] = STATES.idle[key];
    this.cur.dir = STATES.idle.dir;
    this.deriveInk();
    // The loop is the pet: without it a 2D character paints only when a still frame or a
    // state change asks, which leaves an animated idle blank.
    this.start();
  }

  protected onTheme(): void {
    this.deriveInk();
  }

  protected onEnter(s: PetState): void {
    this.cur.dir = STATES[s].dir;
    if (s === "success") {
      this.flash = 1;
      this.sweepX = 0;
    }
    if (s === "error") this.flash = 1;
    this.deriveInk();
  }

  protected onPulse(): void {
    // A token leaving the top of the flame.
    const s = this.shape(this.cur);
    for (let i = 0; i < 2; i++) {
      this.escape.push({
        x: s.cx + (Math.random() - 0.5) * s.rx * 0.9,
        y: s.cy - s.ry * 0.55,
        vy: -18 - Math.random() * 22,
        vx: (Math.random() - 0.5) * 14,
        life: 1,
        ch: GLYPHS[(Math.random() * GLYPHS.length) | 0],
        seed: Math.random(),
      });
    }
  }

  protected onSize(): void {
    this.build();
  }

  /** A still frame sits exactly on the state's preset: no interpolation left running. */
  protected snap(): void {
    const preset = STATES[this._state];
    for (const key of NUMBERS) this.cur[key] = preset[key];
    this.cur.dir = preset.dir;
    this.flash = 0;
    this.sweepX = -1;
    this.escape = [];
  }

  private deriveInk(): void {
    const c = STATES[this._state].c;
    this.tri = hex(c);
    this.ink = this.paintOf(c);
    this.hot = this.paintOf(c, 0.8);
  }

  /** The ink is the identity; the theme decides whether the phosphor lights or prints. */
  private paintOf(c: string, lift = 0): string {
    const k = hex(c);
    return rgb(
      this.theme === "dark"
        ? mix(k, [235, 255, 242], 0.28 + lift * 0.55)
        : mix(k, [6, 12, 9], 0.38 - lift * 0.3),
    );
  }

  private glyphAlpha(a: number): string {
    const t = this.tri;
    return `rgba(${t[0]},${t[1]},${t[2]},${a})`;
  }

  private build(): void {
    const cols = Math.max(10, Math.ceil(this.w / this.step) + 1);
    const rows = Math.max(8, Math.ceil(this.h / this.cell) + 2);
    this.rows = rows;
    this.cols = [];
    for (let i = 0; i < cols; i++) {
      const g = new Array<string>(rows);
      for (let j = 0; j < rows; j++) g[j] = GLYPHS[(Math.random() * GLYPHS.length) | 0];
      this.cols.push({
        x: i * this.step,
        y: Math.random() * rows,
        speed: 0.55 + Math.random() * 0.9,
        len: Math.round(rows * (0.3 + Math.random() * 0.4)),
        g,
        seed: Math.random(),
      });
    }
  }

  /** The mass has no outline: soft lobes whose density decides where rain is body and where
   *  it is only the background. */
  private shape(c: Record<string, number>) {
    const h = this.h;
    return {
      cx: this.w / 2 + this.lookC.x * this.w * 0.03,
      cy: h / 2 - this.lookC.y * h * 0.05,
      rx: h * 0.31 * c.w,
      ry: h * 0.47 * c.h,
    };
  }

  private densityAt(
    x: number,
    y: number,
    s: { cx: number; cy: number; rx: number; ry: number },
  ): number {
    const dx = (x - s.cx) / s.rx;
    const dy = (y - s.cy) / s.ry;
    const r = Math.sqrt(dx * dx + dy * dy * 1.05);
    const t = Math.max(0, Math.min(1, (r - 0.42) / 0.62));
    return 1 - t * t * (3 - 2 * t);
  }

  protected tick(dt: number): void {
    const preset = STATES[this._state];
    const k = 1 - Math.exp(-dt * 4);
    const c = this.cur;
    for (const key of NUMBERS) c[key] += (preset[key] - c[key]) * k;
    this.flash *= Math.exp(-dt * 2.4);

    if (preset.sweep && this.sweepX >= 0) this.sweepX += dt * this.w * 1.3;
    if (this.sweepX > this.w * 1.2) this.sweepX = -1;

    for (let i = this.escape.length - 1; i >= 0; i--) {
      const e = this.escape[i];
      e.x += e.vx * dt;
      e.y += e.vy * dt;
      e.vy -= 16 * dt;
      e.life -= dt * 0.9;
      if (e.life <= 0) this.escape.splice(i, 1);
    }

    this.draw(dt, preset, c);
  }

  private draw(dt: number, P: Preset, c: Record<string, number>): void {
    const ctx = this.ctx;
    const w = this.w;
    const cell = this.cell;
    ctx.font = `500 ${cell}px ${FONT}`;
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";

    const s = this.shape(c);

    // The halo: the mass is felt before it is read.
    const glow = ctx.createRadialGradient(s.cx, s.cy, 0, s.cx, s.cy, s.ry * 1.15);
    glow.addColorStop(0, this.glyphAlpha(0.2 * c.alpha));
    glow.addColorStop(0.5, this.glyphAlpha(0.08 * c.alpha));
    glow.addColorStop(1, this.glyphAlpha(0));
    ctx.fillStyle = glow;
    ctx.save();
    ctx.translate(s.cx, s.cy);
    ctx.scale(1, s.ry / s.rx);
    ctx.beginPath();
    ctx.arc(0, 0, s.rx * 1.15, 0, TAU);
    ctx.fill();
    ctx.restore();

    // The rain.
    for (const col of this.cols) {
      if (Math.abs(col.x - s.cx) > s.rx * 1.5) continue;
      col.y += c.dir * col.speed * c.speed * dt * (this.rows * 0.55);
      if (col.y > this.rows + col.len) col.y = -col.len * Math.random();
      if (col.y < -col.len) col.y = this.rows + col.len * Math.random();

      const jitter = P.jit > 0 ? (Math.random() - 0.5) * c.jit * 16 * (this.flash + 0.35) : 0;
      const head = Math.round(col.y);
      // Toward the centre the columns are longer: the core is full, the edge fades.
      const core = 1 - Math.min(1, Math.abs(col.x - s.cx) / (s.rx * 1.5));
      const len = Math.round(col.len * (0.75 + core * 0.9));

      for (let j = 0; j < len; j++) {
        const row = Math.round(head - c.dir * j);
        if (row < 0 || row >= this.rows) continue;
        const y = row * cell + cell / 2;
        const x = col.x + jitter;
        let glyph = col.g[row];
        if (!glyph) continue;
        if (j === 0) {
          glyph = GLYPHS[(Math.random() * GLYPHS.length) | 0];
          col.g[row] = glyph;
        }

        let a = Math.pow(1 - j / len, 1.05) * 1.02 * c.alpha;
        const dens = this.densityAt(x, y, s);
        a *= 0.03 + 0.97 * dens * dens;
        if (P.jit > 0 && col.seed < c.jit * 0.45) a *= 0.3;
        if (this.sweepX >= 0) {
          a = Math.min(1, a + Math.exp(-Math.pow((x - this.sweepX) / (w * 0.05), 2)) * 0.95);
        }
        ctx.globalAlpha = Math.max(0, a);
        ctx.fillStyle = j === 0 ? this.hot : this.ink;
        ctx.fillText(glyph, x, y);
      }
    }
    ctx.globalAlpha = 1;

    // ---- The glasses ---------------------------------------------------------
    // Two rimless lenses: the glass removes the rain behind it and then darkens itself, so
    // it is neither an invisible hole nor a drawn silhouette. The bridge, the arm and the
    // light on the glass are the details that make them read as glasses. The expression
    // stays in the interpolated numbers: opening, tilt, light.
    const base = Math.max(3, s.rx * 0.22);
    const blinkT = this.t % 4.4;
    const blink = blinkT < 0.17 ? Math.sin((blinkT / 0.17) * Math.PI) : 0;
    const squint = Math.min(1, this.energy * 0.55);
    const lw = base * 1.3 * c.eyeW;
    const lh = Math.max(0.9, base * 1.0 * c.eyeH * (1 - blink * 0.42) * (1 - squint * 0.2));
    const ex = lw + base * 0.38;
    const ey = s.cy - s.ry * 0.24 + s.ry * c.eyeDy;
    const gaze = Math.sin(this.t * 0.55) * base * 0.3 * c.eyeGaze;
    const jitEye =
      this._state === "working"
        ? (Math.random() - 0.5) * 1.3
        : P.jit > 0
          ? (Math.random() - 0.5) * 2.6
          : 0;
    const tilt = c.eyeTilt;

    // The lens profile: a trapezoid with rounded corners. The nose side is short and almost
    // straight, the outer side tall and round, the top rising outward. It is the shape of
    // Neo's lens: tapered at the sides, never pointed.
    const lensPath = (lw: number, lh: number): void => {
      const points = [
        { x: -lw, y: -lh * 0.72, r: lh * 0.34 },
        { x: lw, y: -lh * 0.9, r: lh * 0.8 },
        { x: lw, y: lh * 0.52, r: lh * 0.8 },
        { x: -lw * 0.88, y: lh * 0.08, r: lh * 0.34 },
      ];
      const cut = points.map((cur, i) => {
        const nxt = points[(i + 1) % points.length];
        const prv = points[(i + points.length - 1) % points.length];
        const d1 = Math.hypot(cur.x - prv.x, cur.y - prv.y) || 1;
        const d2 = Math.hypot(nxt.x - cur.x, nxt.y - cur.y) || 1;
        const r = Math.min(cur.r, d1 * 0.5, d2 * 0.5);
        return {
          cur,
          a: { x: cur.x + ((prv.x - cur.x) / d1) * r, y: cur.y + ((prv.y - cur.y) / d1) * r },
          b: { x: cur.x + ((nxt.x - cur.x) / d2) * r, y: cur.y + ((nxt.y - cur.y) / d2) * r },
        };
      });
      ctx.beginPath();
      ctx.moveTo(cut[0].b.x, cut[0].b.y);
      for (let k = 1; k <= cut.length; k++) {
        const point = cut[k % cut.length];
        ctx.lineTo(point.a.x, point.a.y);
        ctx.quadraticCurveTo(point.cur.x, point.cur.y, point.b.x, point.b.y);
      }
      ctx.closePath();
    };

    for (const side of [-1, 1]) {
      const x0 = s.cx + side * ex + gaze + jitEye;
      const y0 = ey + jitEye * 0.5;
      ctx.save();
      ctx.translate(x0, y0);
      ctx.scale(side, 1); // +x is the outer side, for both
      ctx.rotate(tilt);
      ctx.lineJoin = "round";
      ctx.lineCap = "round";

      // The glass removes the rain: behind the lens it does not rain.
      ctx.globalCompositeOperation = "destination-out";
      lensPath(lw, lh);
      ctx.fillStyle = "#000";
      ctx.fill();
      ctx.globalCompositeOperation = "source-over";

      // And it darkens itself: on the light and the dark theme the lens is black.
      const glass = ctx.createLinearGradient(0, -lh * 1.15, 0, lh);
      glass.addColorStop(0, "rgba(12,24,18,.58)");
      glass.addColorStop(0.55, "rgba(6,16,11,.80)");
      glass.addColorStop(1, "rgba(2,8,5,.92)");
      lensPath(lw, lh);
      ctx.fillStyle = glass;
      ctx.fill();

      // Inside the glass: the curvature, the long reflection, a shard of light.
      ctx.save();
      lensPath(lw, lh);
      ctx.clip();

      ctx.globalAlpha = 0.05 + 0.07 * c.eyeGlint;
      const curv = ctx.createRadialGradient(
        lw * 0.1,
        -lh * 0.58,
        0,
        lw * 0.1,
        -lh * 0.58,
        lw * 1.2,
      );
      curv.addColorStop(0, "rgba(224,255,238,1)");
      curv.addColorStop(1, "rgba(224,255,238,0)");
      ctx.fillStyle = curv;
      ctx.fillRect(-lw * 1.1, -lh * 1.3, lw * 2.2, lh * 2.6);
      ctx.globalAlpha = 1;

      ctx.globalAlpha = 0.09 + 0.4 * c.eyeGlint;
      ctx.fillStyle = this.hot;
      ctx.save();
      ctx.translate(lw * 0.26, -lh * 0.34);
      ctx.rotate(-0.34);
      ctx.beginPath();
      ctx.ellipse(0, 0, lw * 0.4, Math.max(0.6, lh * 0.15), 0, 0, TAU);
      ctx.fill();
      ctx.restore();
      ctx.globalAlpha = 0.05 + 0.16 * c.eyeGlint;
      ctx.beginPath();
      ctx.ellipse(lw * 0.58, lh * 0.2, lw * 0.13, Math.max(0.5, lh * 0.08), -0.24, 0, TAU);
      ctx.fill();

      // The glass curvature: light slides inside the top edge.
      ctx.globalAlpha = 0.05 + 0.13 * c.eyeGlint;
      ctx.strokeStyle = this.hot;
      ctx.lineWidth = 3;
      ctx.save();
      ctx.translate(0, 1.6);
      lensPath(lw, lh);
      ctx.stroke();
      ctx.restore();
      ctx.globalAlpha = 1;

      // success: light crosses the glass, once.
      if (P.sweep > 0.02) {
        const ph = ((this.t * 0.62) % 1.5) / 1.5;
        const bx = -lw * 1.5 + ph * lw * 3.2;
        ctx.globalAlpha = 0.5 * P.sweep;
        ctx.save();
        ctx.rotate(-0.28);
        const band = ctx.createLinearGradient(bx - lw * 0.55, 0, bx + lw * 0.55, 0);
        band.addColorStop(0, "rgba(255,255,255,0)");
        band.addColorStop(0.5, "rgba(255,255,255,.95)");
        band.addColorStop(1, "rgba(255,255,255,0)");
        ctx.fillStyle = band;
        ctx.fillRect(bx - lw * 0.55, -lh * 2.2, lw * 1.1, lh * 4.4);
        ctx.restore();
        ctx.globalAlpha = 1;
      }

      // error: the lens cracks, and the crack lights up.
      if (c.eyeCrack > 0.02) {
        ctx.globalAlpha = c.eyeCrack * 0.8;
        ctx.strokeStyle = "rgba(255,244,240,.95)";
        ctx.lineWidth = Math.max(0.7, base * 0.07);
        ctx.beginPath();
        ctx.moveTo(-lw * 0.1, -lh * 1.15);
        ctx.lineTo(-lw * 0.32, -lh * 0.18);
        ctx.lineTo(-lw * 0.1, lh * 0.02);
        ctx.lineTo(-lw * 0.28, lh * 1.0);
        ctx.stroke();
        ctx.globalAlpha = c.eyeCrack * 0.55;
        ctx.lineWidth = Math.max(0.6, base * 0.05);
        ctx.beginPath();
        ctx.moveTo(-lw * 0.31, -lh * 0.2);
        ctx.lineTo(lw * 0.14, -lh * 0.52);
        ctx.moveTo(-lw * 0.11, lh * 0.02);
        ctx.lineTo(lw * 0.2, lh * 0.26);
        ctx.stroke();
        ctx.globalAlpha = 1;
      }
      ctx.restore();

      // The rim: without a frame the line of light sits only on the cap, where the glass
      // curves; the rest is a shadow, which is what gives thickness.
      ctx.strokeStyle = "rgba(0,0,0,.58)";
      ctx.lineWidth = 2.2;
      lensPath(lw, lh);
      ctx.stroke();

      ctx.globalAlpha = Math.max(0, Math.min(1, 0.1 + c.eyeRim * 0.28));
      ctx.strokeStyle = this.hot;
      ctx.lineWidth = 0.9;
      lensPath(lw, lh);
      ctx.stroke();

      ctx.save();
      ctx.beginPath();
      ctx.rect(-lw * 1.7, -lh * 2.4, lw * 3.4, lh * 2.25);
      ctx.clip();
      ctx.globalAlpha = Math.max(0, Math.min(1, 0.06 + c.eyeRim * 0.86));
      ctx.lineWidth = 1.1;
      lensPath(lw, lh);
      ctx.stroke();
      ctx.restore();
      ctx.globalAlpha = 1;

      ctx.restore();
    }

    // The tokens leaving the top.
    for (const e of this.escape) {
      ctx.globalAlpha = Math.max(0, e.life) * 0.85;
      ctx.fillStyle = this.hot;
      ctx.fillText(e.ch, e.x, e.y);
    }
    ctx.globalAlpha = 1;
  }
}
