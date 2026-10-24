import * as THREE from "three";
import { PetBase, WHITE, col } from "../PetBase";
import { PET_STATES, type PetOptions, type PetState } from "../types";

interface Preset {
  a: string;
  b: string;
  spread: number;
  twist: number;
  rotS: number;
  wave: number;
  glow: number;
  size: number;
  breath: number;
  bob: number;
  bobS: number;
  eye: number;
  shrink: number;
  scanOn: number;
  spd: number;
  pull: number;
}

// The character's palette: a categorical identity, like a graph cluster's colour, so
// these hex values stay raw instead of becoming semantic tokens.
const STATES: Record<PetState, Preset> = {
  idle: {
    a: "#8affc1",
    b: "#5aa9ff",
    spread: 0.035,
    twist: 0,
    rotS: 0.15,
    wave: 0.01,
    glow: 0.95,
    size: 0.06,
    breath: 0.03,
    bob: 0.05,
    bobS: 1.2,
    eye: 1,
    shrink: 1,
    scanOn: 0,
    spd: 0.6,
    pull: 0.5,
  },
  listening: {
    a: "#ffffff",
    b: "#7cf2ff",
    spread: 0.02,
    twist: 0,
    rotS: 0.05,
    wave: 0,
    glow: 1.17,
    size: 0.064,
    breath: 0.015,
    bob: 0.03,
    bobS: 1.6,
    eye: 1.25,
    shrink: 0.93,
    scanOn: 0,
    spd: 0.7,
    pull: 1.7,
  },
  thinking: {
    a: "#c79bff",
    b: "#6a7bff",
    spread: 0.05,
    twist: 1.6,
    rotS: 1.1,
    wave: 0.03,
    glow: 1.08,
    size: 0.06,
    breath: 0.02,
    bob: 0.04,
    bobS: 2.2,
    eye: 0.55,
    shrink: 0.96,
    scanOn: 0,
    spd: 1.4,
    pull: 0.4,
  },
  working: {
    a: "#ffd166",
    b: "#ff7a59",
    spread: 0.06,
    twist: 0.3,
    rotS: 0.5,
    wave: 0.05,
    glow: 1.12,
    size: 0.058,
    breath: 0.02,
    bob: 0.05,
    bobS: 3.2,
    eye: 0.9,
    shrink: 1,
    scanOn: 1,
    spd: 2.4,
    pull: 0.4,
  },
  speaking: {
    a: "#8affc1",
    b: "#ffe27a",
    spread: 0.04,
    twist: 0,
    rotS: 0.2,
    wave: 0.14,
    glow: 1.17,
    size: 0.064,
    breath: 0.03,
    bob: 0.05,
    bobS: 2.6,
    eye: 1,
    shrink: 1,
    scanOn: 0,
    spd: 2.2,
    pull: 0.5,
  },
  success: {
    a: "#fff3a0",
    b: "#8affc1",
    spread: 0.04,
    twist: 0,
    rotS: 0.4,
    wave: 0.02,
    glow: 1.44,
    size: 0.068,
    breath: 0.03,
    bob: 0.05,
    bobS: 2.0,
    eye: 1.1,
    shrink: 1,
    scanOn: 0,
    spd: 1.4,
    pull: 0.5,
  },
  error: {
    a: "#ff5b6e",
    b: "#ff9f6e",
    spread: 0.08,
    twist: 0,
    rotS: 0.05,
    wave: 0.03,
    glow: 1.2,
    size: 0.064,
    breath: 0.02,
    bob: 0.02,
    bobS: 2.0,
    eye: 0.8,
    shrink: 1,
    scanOn: 0,
    spd: 3.0,
    pull: 0.2,
  },
  sleeping: {
    a: "#5a6bb0",
    b: "#3a4a8a",
    spread: 0.02,
    twist: 0,
    rotS: 0.03,
    wave: 0,
    glow: 0.72,
    size: 0.05,
    breath: 0.05,
    bob: 0.02,
    bobS: 0.5,
    eye: 0.04,
    shrink: 0.82,
    scanOn: 0,
    spd: 0.2,
    pull: 0.1,
  },
};
const NUM = [
  "spread",
  "twist",
  "rotS",
  "wave",
  "glow",
  "size",
  "breath",
  "bob",
  "bobS",
  "eye",
  "shrink",
  "scanOn",
  "spd",
  "pull",
] as const;
type NumKey = (typeof NUM)[number];

const VERT = /* glsl */ `
attribute vec3 aDir; attribute vec3 aSeed; attribute float aR;
uniform float uTime,uBreath,uSpread,uTwist,uRot,uWave,uBurst,uJit,uSize,uScale,uScan,uScanOn,uShrink;
uniform vec2 uLook;
varying float vMix, vBoost, vHole;
void main(){
  vec3 p = aDir * aR * uShrink * (1.0 + uBreath);
  p += sin(aSeed*6.2831 + uTime*vec3(1.0,1.3,.8) + aSeed.yzx*4.0) * uSpread;
  p *= 1.0 + uWave*sin(p.y*5.0 - uTime*2.0 + aSeed.x*6.2831);
  float ang = uTwist*p.y + uRot;
  float c = cos(ang), s = sin(ang);
  p.xz = mat2(c,-s,s,c)*p.xz;
  p += aDir * uBurst * (.4 + aSeed.y);
  p += sin(aSeed*40.0 + uTime*25.0) * uJit * .09;
  p.xy += uLook * .22 * (aDir.z*.5 + .5);
  vec2 q1 = vec2((p.x + .3)/.14, (p.y - .14)/.21), q2 = vec2((p.x - .3)/.14, (p.y - .14)/.21);
  vHole = mix(1.0, smoothstep(1.0, 2.5, min(length(q1), length(q2))), step(0.0, p.z));
  vec4 mv = modelViewMatrix * vec4(p,1.0);
  gl_Position = projectionMatrix * mv;
  gl_PointSize = uSize * uScale * (.55 + aSeed.z*.9) / -mv.z;
  vMix = aDir.y*.5 + .5 + .15*sin(uTime*.5 + aDir.x*2.0);
  vBoost = uScanOn * smoothstep(.22, .0, abs(p.y - uScan));
}`;
const FRAG = /* glsl */ `
uniform vec3 uA, uB; uniform float uGlow;
varying float vMix, vBoost, vHole;
void main(){
  float d = length(gl_PointCoord - .5) * 2.0;
  if(d > 1.0) discard;
  float a = 1.0 - smoothstep(.45, 1.0, d);
  vec3 col = mix(uB, uA, clamp(vMix,0.,1.));
  col = mix(col, vec3(1.0), vBoost*.8);
  gl_FragColor = vec4(col, a*min(uGlow + vBoost, 1.4)*.8*mix(.06, 1.0, vHole));
}`;

/**
 * Variant 2: the swarm. A swarm of luminous points (additive): it reads best on a dark
 * background.
 */
export class SciamePet extends PetBase {
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
  private u: Record<string, THREE.IUniform>;
  private root = new THREE.Group();
  private eyes: THREE.Group[];
  private haloMat: THREE.MeshBasicMaterial;
  private pupilMat: THREE.MeshBasicMaterial;
  private phase = 0;
  private rot = 0;
  private burst = 0;
  private jit = 0;
  private nextBlink = 2;
  private blink = 0;

  constructor(canvas: HTMLCanvasElement, opts: PetOptions = {}) {
    super(canvas, opts);
    this.camera.position.z = 5.2;

    // the swarm: uniform directions on a sphere, radius concentrated near the surface
    const N = Math.max(400, Math.round(3200 * (opts.density ?? 1)));
    const dir = new Float32Array(N * 3);
    const seed = new Float32Array(N * 3);
    const rad = new Float32Array(N);
    for (let i = 0; i < N; i++) {
      const z = Math.random() * 2 - 1;
      const t = Math.random() * Math.PI * 2;
      const q = Math.sqrt(1 - z * z);
      dir.set([q * Math.cos(t), z, q * Math.sin(t)], i * 3);
      seed.set([Math.random(), Math.random(), Math.random()], i * 3);
      rad[i] = 1 - Math.pow(Math.random(), 3) * 0.3;
    }
    const g = new THREE.BufferGeometry();
    g.setAttribute("position", new THREE.BufferAttribute(new Float32Array(N * 3), 3));
    g.setAttribute("aDir", new THREE.BufferAttribute(dir, 3));
    g.setAttribute("aSeed", new THREE.BufferAttribute(seed, 3));
    g.setAttribute("aR", new THREE.BufferAttribute(rad, 1));

    const U = <T>(value: T): THREE.IUniform<T> => ({ value });
    this.u = {
      uTime: U(0),
      uBreath: U(0),
      uSpread: U(0.035),
      uTwist: U(0),
      uRot: U(0),
      uWave: U(0),
      uBurst: U(0),
      uJit: U(0),
      uSize: U(0.06),
      uScale: U(400),
      uScan: U(0),
      uScanOn: U(0),
      uShrink: U(1),
      uLook: U(new THREE.Vector2()),
      uA: U(this.cA),
      uB: U(this.cB),
      uGlow: U(0.95),
    };
    const mat = new THREE.ShaderMaterial({
      uniforms: this.u,
      vertexShader: VERT,
      fragmentShader: FRAG,
      transparent: true,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    });
    const pts = new THREE.Points(g, mat);
    pts.frustumCulled = false;
    this.root.add(pts);

    // eyes: a light ellipse with a soft halo (a radial texture), painted over the
    // thinned swarm
    const cv = document.createElement("canvas");
    cv.width = cv.height = 128;
    const cx = cv.getContext("2d")!;
    const gr = cx.createRadialGradient(64, 64, 0, 64, 64, 64);
    gr.addColorStop(0, "rgba(255,255,255,1)");
    gr.addColorStop(0.3, "rgba(255,255,255,.6)");
    gr.addColorStop(1, "rgba(255,255,255,0)");
    cx.fillStyle = gr;
    cx.fillRect(0, 0, 128, 128);
    this.haloMat = new THREE.MeshBasicMaterial({
      map: new THREE.CanvasTexture(cv),
      transparent: true,
      opacity: 0.25,
      blending: THREE.AdditiveBlending,
      depthTest: false,
      depthWrite: false,
    });
    this.pupilMat = new THREE.MeshBasicMaterial({
      color: col(0xffffff),
      transparent: true,
      depthTest: false,
      depthWrite: false,
    });
    const ball = new THREE.SphereGeometry(1, 28, 18);
    const quad = new THREE.PlaneGeometry(1, 1);
    this.eyes = [-1, 1].map((s) => {
      const grp = new THREE.Group();
      const h = new THREE.Mesh(quad, this.haloMat);
      h.scale.set(0.3, 0.42, 1);
      h.renderOrder = 9;
      const p = new THREE.Mesh(ball, this.pupilMat);
      p.scale.set(0.085, 0.14, 0.02);
      p.renderOrder = 10;
      grp.add(h, p);
      const x = s * 0.3;
      const y = 0.14;
      grp.position.set(x, y, Math.sqrt(1 - x * x - y * y) * 1.02);
      this.root.add(grp);
      return grp;
    });
    this.scene.add(this.root);
    this.start();
  }

  protected onEnter(s: PetState): void {
    if (s === "success") this.burst = 1; // the swarm bursts and reassembles
    if (s === "error") this.jit = 1; // the swarm shivers and comes apart
  }

  protected onResize(_w: number, h: number): void {
    this.u.uScale.value = h * this.renderer.getPixelRatio() * 0.5;
  }

  /**
   * Puts the interpolation and the transient effects on the current preset: a still swarm
   * is the state's own shape, with no burst and no jitter.
   */
  protected snap(): void {
    const P = STATES[this.state];
    for (const n of NUM) this.cur[n] = P[n];
    this.cA.copy(this.palA[this.state]);
    this.cB.copy(this.palB[this.state]);
    this.burst = 0;
    this.jit = 0;
    this.phase = 0;
    this.rot = 0;
  }

  protected tick(dt: number): void {
    const P = STATES[this.state];
    const k = 1 - Math.exp(-dt * 5);
    const u = this.u;
    const c = this.cur;
    for (const n of NUM) c[n] += (P[n] - c[n]) * k;
    this.cA.lerp(this.palA[this.state], k);
    this.cB.lerp(this.palB[this.state], k);
    this.burst *= Math.exp(-dt * 2.2);
    this.jit *= Math.exp(-dt * 2.6);
    this.phase += dt * c.bobS;
    this.rot += dt * c.rotS;

    u.uTime.value += dt * c.spd;
    u.uBreath.value = Math.sin(this.phase * 0.8) * c.breath;
    u.uSpread.value = c.spread;
    u.uTwist.value = c.twist;
    u.uRot.value = this.rot;
    u.uWave.value = c.wave + this.energy * 0.06;
    u.uBurst.value = this.burst * 1.3 + this.energy * 0.05;
    u.uJit.value = this.jit;
    u.uSize.value = c.size + this.energy * 0.014;
    u.uGlow.value = c.glow + this.energy * 0.15;
    u.uShrink.value = c.shrink;
    u.uScanOn.value = c.scanOn;
    u.uScan.value = Math.sin(this.t * 2.2) * 0.95;

    const age = this.t - this.t0;
    let x = 0;
    let y = Math.sin(this.phase) * c.bob;
    if (this.state === "success" && age < 1)
      y += Math.abs(Math.sin(age * Math.PI * 2.5)) * 0.4 * (1 - age);
    if (this.state === "error" && age < 0.8) x = Math.sin(age * 55) * 0.05 * (1 - age / 0.8);
    this.root.position.set(x, y, 0);

    (u.uLook.value as THREE.Vector2).set(this.lookC.x, this.lookC.y).multiplyScalar(c.pull);
    this.root.rotation.y = this.lookC.x * 0.35;
    this.root.rotation.x = -this.lookC.y * 0.22;

    if (this.t > this.nextBlink) {
      this.blink = this.t;
      this.nextBlink = this.t + 2 + Math.random() * 3;
    }
    const b = this.t - this.blink;
    const close = b < 0.16 ? Math.sin((b / 0.16) * Math.PI) : 0;
    const open = Math.max(0.05, c.eye * (1 - close * 0.95));
    for (const e of this.eyes) e.scale.set(1, open, 1);

    // light eyes, faintly tinted with the state's mid tone
    this.haloMat.color.copy(this.cA).lerp(this.cB, 0.5).lerp(WHITE, 0.3);
    this.pupilMat.color.copy(this.cA).lerp(this.cB, 0.5).lerp(WHITE, 0.72);
  }
}
