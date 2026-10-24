import * as THREE from "three";
import { PetBase, WHITE, col } from "../PetBase";
import { PET_STATES, type PetOptions, type PetState } from "../types";

type Mode = "rand" | "up" | "look";

interface Preset {
  a: string;
  b: string;
  spread: number;
  rotS: number;
  shrink: number;
  breath: number;
  spd: number;
  size: number;
  edge: number;
  rate: number;
  ps: number;
  chain: number;
  wave: number;
  bob: number;
  bobS: number;
  gaze: number;
  /** the pulses' direction: rand = random, up = bottom to top, look = toward the pointer */
  mode: Mode;
}

// The character's palette: a categorical identity, like a graph cluster's colour, so
// these hex values stay raw instead of becoming semantic tokens.
const STATES: Record<PetState, Preset> = {
  idle: {
    a: "#f2e4c9",
    b: "#e9a36b",
    spread: 0.05,
    rotS: 0.12,
    shrink: 1,
    breath: 0.03,
    spd: 0.5,
    size: 1,
    edge: 0.3,
    rate: 1.2,
    ps: 0.5,
    chain: 0.4,
    wave: 0,
    bob: 0.05,
    bobS: 1.2,
    gaze: 0.5,
    mode: "rand",
  },
  listening: {
    a: "#ffffff",
    b: "#9ad8ff",
    spread: 0.03,
    rotS: 0.04,
    shrink: 0.95,
    breath: 0.015,
    spd: 0.6,
    size: 1.15,
    edge: 0.42,
    rate: 3,
    ps: 0.9,
    chain: 0.7,
    wave: 0,
    bob: 0.03,
    bobS: 1.6,
    gaze: 1.2,
    mode: "look",
  },
  thinking: {
    a: "#c9b8ff",
    b: "#6f8cff",
    spread: 0.07,
    rotS: 0.7,
    shrink: 0.98,
    breath: 0.02,
    spd: 1.6,
    size: 1,
    edge: 0.4,
    rate: 16,
    ps: 1.3,
    chain: 0.7,
    wave: 0,
    bob: 0.04,
    bobS: 2.2,
    gaze: 0.2,
    mode: "rand",
  },
  working: {
    a: "#ffd18a",
    b: "#ff8a5c",
    spread: 0.05,
    rotS: 0.35,
    shrink: 1,
    breath: 0.02,
    spd: 2,
    size: 1.05,
    edge: 0.4,
    rate: 11,
    ps: 1.6,
    chain: 0.85,
    wave: 0,
    bob: 0.05,
    bobS: 3.2,
    gaze: 0.2,
    mode: "up",
  },
  speaking: {
    a: "#ffe9b0",
    b: "#f2a65a",
    spread: 0.05,
    rotS: 0.15,
    shrink: 1,
    breath: 0.03,
    spd: 2,
    size: 1.1,
    edge: 0.45,
    rate: 4,
    ps: 1.2,
    chain: 0.5,
    wave: 0.55,
    bob: 0.05,
    bobS: 2.6,
    gaze: 0.4,
    mode: "rand",
  },
  success: {
    a: "#fff3b0",
    b: "#9dffcf",
    spread: 0.04,
    rotS: 0.3,
    shrink: 1,
    breath: 0.03,
    spd: 1.3,
    size: 1.2,
    edge: 0.6,
    rate: 6,
    ps: 1,
    chain: 0.5,
    wave: 0,
    bob: 0.05,
    bobS: 2.0,
    gaze: 0.4,
    mode: "rand",
  },
  error: {
    a: "#ff7b7b",
    b: "#b04a6a",
    spread: 0.09,
    rotS: 0.04,
    shrink: 1,
    breath: 0.02,
    spd: 3,
    size: 1,
    edge: 0.3,
    rate: 1,
    ps: 0.8,
    chain: 0.3,
    wave: 0,
    bob: 0.02,
    bobS: 2.0,
    gaze: 0,
    mode: "rand",
  },
  sleeping: {
    a: "#7d86b5",
    b: "#44507f",
    spread: 0.02,
    rotS: 0.03,
    shrink: 0.82,
    breath: 0.05,
    spd: 0.2,
    size: 0.8,
    edge: 0.12,
    rate: 0.2,
    ps: 0.3,
    chain: 0.2,
    wave: 0,
    bob: 0.02,
    bobS: 0.5,
    gaze: 0,
    mode: "rand",
  },
};
const NUM = [
  "spread",
  "rotS",
  "shrink",
  "breath",
  "spd",
  "size",
  "edge",
  "rate",
  "ps",
  "chain",
  "wave",
  "bob",
  "bobS",
  "gaze",
] as const;
type NumKey = (typeof NUM)[number];

const VERT = /* glsl */ `
attribute float aAct; attribute vec3 aCol;
uniform float uSize, uScale, uBase, uBoost;
varying float vAct; varying vec3 vCol;
void main(){
  vec4 mv = modelViewMatrix * vec4(position,1.0);
  gl_Position = projectionMatrix * mv;
  gl_PointSize = uSize * uScale * (uBase + aAct*uBoost) / -mv.z;
  vAct = aAct; vCol = aCol;
}`;
const FRAG = /* glsl */ `
uniform float uAlphaBase;
varying float vAct; varying vec3 vCol;
void main(){
  float d = length(gl_PointCoord - .5) * 2.0;
  if(d > 1.0) discard;
  float a = pow(1.0 - d, 1.6);
  vec3 col = mix(vCol, vec3(1.0), pow(1.0 - d, 4.0) * (.35 + .5*clamp(vAct,0.,1.)));
  gl_FragColor = vec4(col, a * (uAlphaBase + (1.0 - uAlphaBase) * clamp(vAct,0.,1.)));
}`;

const MAXP = 140;
interface Pulse {
  i: number;
  j: number;
  t: number;
  v: number;
}

/**
 * Variant 3: the weave. A network of nodes and connections crossed by pulses of attention.
 * Additive: it reads best on a dark background.
 */
export class TramaPet extends PetBase {
  protected readonly once = { success: 1.8, error: 1.3 };

  private cur = Object.fromEntries(NUM.map((k) => [k, STATES.idle[k]])) as Record<NumKey, number>;
  private palA = Object.fromEntries(PET_STATES.map((s) => [s, col(STATES[s].a)])) as Record<
    PetState,
    THREE.Color
  >;
  private palB = Object.fromEntries(PET_STATES.map((s) => [s, col(STATES[s].b)])) as Record<
    PetState,
    THREE.Color
  >;
  private cA = col(STATES.idle.a);
  private cB = col(STATES.idle.b);
  private cc = new THREE.Color();

  private N: number;
  private base: THREE.Vector3[] = [];
  private seed: THREE.Vector3[] = [];
  private bl: number[] = [];
  private edges: [number, number][] = [];
  private adj: number[][];

  private pos: Float32Array;
  private aAct: Float32Array;
  private nAct: Float32Array;
  private nCol: Float32Array;
  private ePos: Float32Array;
  private eCol: Float32Array;
  private pPos = new Float32Array(MAXP * 3);
  private pAct = new Float32Array(MAXP);
  private pCol = new Float32Array(MAXP * 3);
  private dyn: THREE.BufferAttribute[] = [];
  private nodeMat: THREE.ShaderMaterial;
  private uScale: THREE.IUniform<number> = { value: 600 };
  private root = new THREE.Group();

  private pulses: Pulse[] = [];
  private acc = 0;
  private gz = new THREE.Vector3();
  private mode: Mode = "rand";
  private ut = 0;
  private phase = 0;
  private rot = 0;
  private flash = 0;
  private jit = 0;

  constructor(canvas: HTMLCanvasElement, opts: PetOptions = {}) {
    super(canvas, opts);
    this.camera.position.z = 5.2;

    // nodes on an irregular sphere (Fibonacci points with a varying radius)
    const N = (this.N = Math.max(40, Math.round(120 * (opts.density ?? 1))));
    for (let i = 0; i < N; i++) {
      const y = 1 - ((i + 0.5) / N) * 2;
      const r = Math.sqrt(1 - y * y);
      const th = i * 2.39996;
      const rr = 0.8 + Math.random() * 0.35;
      const b = new THREE.Vector3(Math.cos(th) * r, y, Math.sin(th) * r).multiplyScalar(rr);
      this.base.push(b);
      this.bl.push(b.length());
      this.seed.push(new THREE.Vector3(Math.random(), Math.random(), Math.random()));
    }

    // connections: every node is bound to its 3 nearest neighbours
    const seen = new Set<number>();
    this.adj = Array.from({ length: N }, () => []);
    for (let i = 0; i < N; i++) {
      const d: [number, number][] = [];
      for (let j = 0; j < N; j++)
        if (j !== i) d.push([this.base[i].distanceToSquared(this.base[j]), j]);
      d.sort((p, q) => p[0] - q[0]);
      for (let k = 0; k < 3; k++) {
        const j = d[k][1];
        const key = i < j ? i * 1000 + j : j * 1000 + i;
        if (!seen.has(key)) {
          seen.add(key);
          this.edges.push([i, j]);
        }
      }
    }
    for (const [i, j] of this.edges) {
      this.adj[i].push(j);
      this.adj[j].push(i);
    }

    const A = (arr: Float32Array, n: number) => {
      const a = new THREE.BufferAttribute(arr, n);
      a.setUsage(THREE.DynamicDrawUsage);
      this.dyn.push(a);
      return a;
    };

    this.pos = new Float32Array(N * 3);
    this.aAct = new Float32Array(N);
    this.nAct = new Float32Array(N);
    this.nCol = new Float32Array(N * 3);
    const ng = new THREE.BufferGeometry();
    ng.setAttribute("position", A(this.pos, 3));
    ng.setAttribute("aAct", A(this.aAct, 1));
    ng.setAttribute("aCol", A(this.nCol, 3));

    this.ePos = new Float32Array(this.edges.length * 6);
    this.eCol = new Float32Array(this.edges.length * 6);
    const eg = new THREE.BufferGeometry();
    eg.setAttribute("position", A(this.ePos, 3));
    eg.setAttribute("color", A(this.eCol, 3));

    const pg = new THREE.BufferGeometry();
    pg.setAttribute("position", A(this.pPos, 3));
    pg.setAttribute("aAct", A(this.pAct, 1));
    pg.setAttribute("aCol", A(this.pCol, 3));

    const mk = (size: number, base: number, boost: number, alphaBase: number) =>
      new THREE.ShaderMaterial({
        vertexShader: VERT,
        fragmentShader: FRAG,
        transparent: true,
        depthWrite: false,
        blending: THREE.AdditiveBlending,
        uniforms: {
          uSize: { value: size },
          uScale: this.uScale,
          uBase: { value: base },
          uBoost: { value: boost },
          uAlphaBase: { value: alphaBase },
        },
      });
    this.nodeMat = mk(0.1, 1, 0.9, 0.55);
    const lines = new THREE.LineSegments(
      eg,
      new THREE.LineBasicMaterial({
        vertexColors: true,
        transparent: true,
        depthWrite: false,
        blending: THREE.AdditiveBlending,
      }),
    );
    const nodes = new THREE.Points(ng, this.nodeMat);
    const pulses = new THREE.Points(pg, mk(0.13, 0, 1, 0));
    for (const o of [lines, nodes, pulses]) o.frustumCulled = false;
    this.root.add(lines, nodes, pulses);
    this.scene.add(this.root);
    this.start();
  }

  protected onEnter(s: PetState): void {
    if (s === "success") this.flash = 1; // the network lights up and reassembles
    if (s === "error") this.jit = 1; // connections snap, nodes shiver
  }

  protected onPulse(a: number): void {
    for (let i = 0, n = Math.round(a * 8); i < n; i++) this.spawn();
  }

  protected onResize(_w: number, h: number): void {
    this.uScale.value = h * this.renderer.getPixelRatio() * 0.5;
  }

  /** Picks the next node along a connection, following the state's direction. */
  private pick(from: number, prev: number): number {
    const c = this.adj[from].filter((n) => n !== prev);
    if (!c.length) return -1;
    let best = -1;
    let bs = -1e9;
    for (const n of c) {
      const b = this.base[n];
      const sc = this.mode === "up" ? b.y : this.mode === "look" ? b.dot(this.gz) : 0;
      const v = sc + Math.random() * (this.mode === "rand" ? 1 : 0.5);
      if (v > bs) {
        bs = v;
        best = n;
      }
    }
    return best;
  }

  private spawn(): void {
    if (this.pulses.length >= MAXP) return;
    const i = (Math.random() * this.N) | 0;
    const j = this.pick(i, -1);
    if (j >= 0) this.pulses.push({ i, j, t: 0, v: 0.8 + Math.random() * 0.5 });
  }

  /**
   * Puts the interpolation and the transient effects on the current preset. The zero-delta
   * tick that follows is what builds the network for the frame: a still trama has to be
   * laid out once, because its geometry is computed in the loop.
   */
  protected snap(): void {
    const P = STATES[this.state];
    for (const n of NUM) this.cur[n] = P[n];
    this.cA.copy(this.palA[this.state]);
    this.cB.copy(this.palB[this.state]);
    this.mode = P.mode;
    this.flash = 0;
    this.jit = 0;
    this.phase = 0;
    this.rot = 0;
    this.ut = 0;
    this.acc = 0;
    this.pulses = [];
  }

  protected tick(dt: number): void {
    const P = STATES[this.state];
    const k = 1 - Math.exp(-dt * 5);
    const c = this.cur;
    const N = this.N;
    for (const n of NUM) c[n] += (P[n] - c[n]) * k;
    this.cA.lerp(this.palA[this.state], k);
    this.cB.lerp(this.palB[this.state], k);
    this.mode = P.mode;
    this.flash *= Math.exp(-dt * 1.8);
    this.jit *= Math.exp(-dt * 2.6);
    this.phase += dt * c.bobS;
    this.rot += dt * c.rotS;
    this.ut += dt * c.spd;

    this.gz.set(this.lookC.x * 0.9, this.lookC.y * 0.9, 0.6).normalize();

    this.acc += dt * (c.rate + this.energy * 6);
    while (this.acc >= 1) {
      this.acc -= 1;
      this.spawn();
    }

    // nodes
    const sh = c.shrink * (1 + Math.sin(this.phase * 0.8) * c.breath) * (1 + this.flash * 0.3);
    const pos = this.pos;
    const ut = this.ut;
    const decay = Math.exp(-dt * 2.5);
    const { cA, cB } = this;
    for (let i = 0; i < N; i++) {
      const b = this.base[i];
      const s = this.seed[i];
      let x = b.x * sh + Math.sin(ut + s.x * 6.283) * c.spread;
      let y = b.y * sh + Math.sin(ut * 1.3 + s.y * 6.283) * c.spread;
      let z = b.z * sh + Math.sin(ut * 0.8 + s.z * 6.283) * c.spread;
      if (this.jit > 0.02) {
        const j = this.jit * 0.14;
        x += (Math.random() - 0.5) * j;
        y += (Math.random() - 0.5) * j;
        z += (Math.random() - 0.5) * j;
      }
      pos[i * 3] = x;
      pos[i * 3 + 1] = y;
      pos[i * 3 + 2] = z;
      this.nAct[i] *= decay;
      const gz = Math.pow(Math.max(0, b.dot(this.gz) / this.bl[i]), 6) * c.gaze;
      const wv = c.wave * (Math.sin(y * 3 - ut * 2.5) * 0.5 + 0.5);
      this.aAct[i] = Math.min(1.6, this.nAct[i] + gz + wv + this.flash * 0.8);
      const tt = Math.min(1, Math.max(0, y * 0.5 + 0.5));
      this.nCol[i * 3] = cB.r + (cA.r - cB.r) * tt;
      this.nCol[i * 3 + 1] = cB.g + (cA.g - cB.g) * tt;
      this.nCol[i * 3 + 2] = cB.b + (cA.b - cB.b) * tt;
    }
    this.nodeMat.uniforms.uSize.value = 0.1 * c.size;

    // connections
    const E = this.edges;
    const ep = this.ePos;
    const ec = this.eCol;
    for (let e = 0; e < E.length; e++) {
      const i = E[e][0];
      const j = E[e][1];
      for (let q = 0; q < 3; q++) {
        ep[e * 6 + q] = pos[i * 3 + q];
        ep[e * 6 + 3 + q] = pos[j * 3 + q];
      }
      let br = c.edge + (this.aAct[i] + this.aAct[j]) * 0.35 + this.flash * 0.5;
      if (this.jit > 0.05 && Math.random() < this.jit * 0.5) br *= 0.05;
      const tt = Math.min(1, Math.max(0, (pos[i * 3 + 1] + pos[j * 3 + 1]) * 0.25 + 0.5));
      const r = (cB.r + (cA.r - cB.r) * tt) * br;
      const g = (cB.g + (cA.g - cB.g) * tt) * br;
      const bl = (cB.b + (cA.b - cB.b) * tt) * br;
      ec[e * 6] = ec[e * 6 + 3] = r;
      ec[e * 6 + 1] = ec[e * 6 + 4] = g;
      ec[e * 6 + 2] = ec[e * 6 + 5] = bl;
    }

    // pulses travelling along the connections
    const pl = this.pulses;
    for (let n = pl.length - 1; n >= 0; n--) {
      const p = pl[n];
      p.t += dt * c.ps * p.v;
      if (p.t >= 1) {
        this.nAct[p.j] = 1;
        const nx = Math.random() < c.chain ? this.pick(p.j, p.i) : -1;
        if (nx >= 0) {
          p.i = p.j;
          p.j = nx;
          p.t = 0;
        } else pl.splice(n, 1);
      }
    }
    this.cc.copy(cA).lerp(WHITE, 0.35);
    for (let n = 0; n < MAXP; n++) {
      const p = pl[n];
      if (p) {
        for (let q = 0; q < 3; q++)
          this.pPos[n * 3 + q] = pos[p.i * 3 + q] + (pos[p.j * 3 + q] - pos[p.i * 3 + q]) * p.t;
        this.pAct[n] = 1;
        this.pCol[n * 3] = this.cc.r;
        this.pCol[n * 3 + 1] = this.cc.g;
        this.pCol[n * 3 + 2] = this.cc.b;
      } else this.pAct[n] = 0;
    }
    for (const a of this.dyn) a.needsUpdate = true;

    const age = this.t - this.t0;
    let x = 0;
    let y = Math.sin(this.phase) * c.bob;
    if (this.state === "success" && age < 1)
      y += Math.abs(Math.sin(age * Math.PI * 2.5)) * 0.4 * (1 - age);
    if (this.state === "error" && age < 0.8) x = Math.sin(age * 55) * 0.05 * (1 - age / 0.8);
    this.root.position.set(x, y, 0);
    this.root.rotation.y = this.rot + this.lookC.x * 0.35;
    this.root.rotation.x = -this.lookC.y * 0.22;
  }
}
