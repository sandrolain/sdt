import * as THREE from "three";
import { PetBase, col } from "../PetBase";
import { PET_STATES, type PetOptions, type PetState } from "../types";

interface Preset {
  c: string;
  amp: number;
  freq: number;
  spd: number;
  glow: number;
  bob: number;
  bobS: number;
  eye: number;
}

// The character's palette: a categorical identity, like a graph cluster's colour, so
// these hex values stay raw instead of becoming semantic tokens.
const STATES: Record<PetState, Preset> = {
  idle: { c: "#8b7cff", amp: 0.06, freq: 1.4, spd: 0.5, glow: 0.5, bob: 0.05, bobS: 1.2, eye: 1 },
  listening: {
    c: "#4cc9ff",
    amp: 0.05,
    freq: 1.6,
    spd: 0.6,
    glow: 0.9,
    bob: 0.03,
    bobS: 1.6,
    eye: 1.25,
  },
  thinking: {
    c: "#b56cff",
    amp: 0.09,
    freq: 2.6,
    spd: 1.6,
    glow: 0.8,
    bob: 0.04,
    bobS: 2.2,
    eye: 0.55,
  },
  working: {
    c: "#ffb347",
    amp: 0.07,
    freq: 2.0,
    spd: 2.4,
    glow: 0.9,
    bob: 0.06,
    bobS: 3.2,
    eye: 0.9,
  },
  speaking: {
    c: "#4ade80",
    amp: 0.07,
    freq: 1.8,
    spd: 1.2,
    glow: 1.0,
    bob: 0.05,
    bobS: 2.6,
    eye: 1,
  },
  success: {
    c: "#ffd84d",
    amp: 0.1,
    freq: 1.6,
    spd: 1.4,
    glow: 1.4,
    bob: 0.05,
    bobS: 2.0,
    eye: 1.1,
  },
  error: {
    c: "#ff5d6c",
    amp: 0.05,
    freq: 3.0,
    spd: 3.0,
    glow: 1.0,
    bob: 0.02,
    bobS: 2.0,
    eye: 0.8,
  },
  sleeping: {
    c: "#4b4f8a",
    amp: 0.03,
    freq: 1.0,
    spd: 0.2,
    glow: 0.15,
    bob: 0.02,
    bobS: 0.5,
    eye: 0.05,
  },
};
const NUM = ["amp", "freq", "spd", "glow", "bob", "bobS", "eye"] as const;
type NumKey = (typeof NUM)[number];

const VERT = /* glsl */ `
uniform float uTime, uAmp, uFreq;
varying vec3 vN, vV;
void main(){
  vec3 p = position;
  float n = sin(p.x*uFreq+uTime)*sin(p.y*uFreq*1.3+uTime*1.1)*sin(p.z*uFreq*.9-uTime*.8);
  p += normal * n * uAmp;
  vec4 mv = modelViewMatrix * vec4(p,1.0);
  vN = normalize(normalMatrix * normal);
  vV = -mv.xyz;
  gl_Position = projectionMatrix * mv;
}`;
const FRAG = /* glsl */ `
uniform vec3 uColor; uniform float uGlow;
varying vec3 vN, vV;
void main(){
  vec3 n = normalize(vN), v = normalize(vV);
  float l = max(dot(n, normalize(vec3(.4,.7,.8))), 0.0);
  float f = pow(1.0 - max(dot(n,v),0.0), 2.2);
  vec3 col = uColor*(.55+.6*l) + uColor*f*uGlow*.9 + vec3(1.0)*pow(f,4.0)*.25*uGlow;
  gl_FragColor = vec4(col,1.0);
}`;

/** Variant 1: a soft, luminous sphere with two eyes. */
export class DotPet extends PetBase {
  protected readonly once = { success: 1.8, error: 1.3 };

  private cur = Object.fromEntries(NUM.map((k) => [k, STATES.idle[k]])) as Record<NumKey, number>;
  private pal = Object.fromEntries(PET_STATES.map((s) => [s, col(STATES[s].c)])) as Record<
    PetState,
    THREE.Color
  >;
  private color = col(STATES.idle.c);
  private u: Record<string, THREE.IUniform>;
  private root = new THREE.Group();
  private eyes: THREE.Mesh[];
  private phase = 0;
  private nextBlink = 2;
  private blink = 0;

  constructor(canvas: HTMLCanvasElement, opts: PetOptions = {}) {
    super(canvas, opts);
    this.camera.position.z = 4.4;

    this.u = {
      uTime: { value: 0 },
      uAmp: { value: 0.06 },
      uFreq: { value: 1.4 },
      uColor: { value: this.color },
      uGlow: { value: 0.5 },
    };
    const body = new THREE.Mesh(
      new THREE.IcosahedronGeometry(1, opts.density && opts.density < 0.6 ? 4 : 5),
      new THREE.ShaderMaterial({ uniforms: this.u, vertexShader: VERT, fragmentShader: FRAG }),
    );
    this.root.add(body);

    const eyeMat = new THREE.MeshBasicMaterial({ color: col(0x101223) });
    const eyeGeo = new THREE.SphereGeometry(1, 20, 14);
    this.eyes = [-1, 1].map((s) => {
      const e = new THREE.Mesh(eyeGeo, eyeMat);
      e.position.set(s * 0.3, 0.14, 0.97);
      e.scale.set(0.085, 0.13, 0.05);
      this.root.add(e);
      return e;
    });
    this.scene.add(this.root);
    this.start();
  }

  /**
   * Puts the interpolation and the shared transient values on the current preset, so a
   * still frame is the state's own shape rather than the tail of a transition.
   */
  protected snap(): void {
    const P = STATES[this.state];
    for (const n of NUM) this.cur[n] = P[n];
    this.color.copy(this.pal[this.state]);
    this.energy = 0;
    this.phase = 0;
    this.blink = this.t;
  }

  protected tick(dt: number): void {
    const P = STATES[this.state];
    const k = 1 - Math.exp(-dt * 6);
    const c = this.cur;
    for (const n of NUM) c[n] += (P[n] - c[n]) * k;
    this.color.lerp(this.pal[this.state], k);

    this.phase += dt * c.bobS;
    this.u.uTime.value += dt * c.spd;
    this.u.uAmp.value = c.amp + this.energy * 0.04;
    this.u.uFreq.value = c.freq;
    this.u.uGlow.value = c.glow + this.energy * 0.3;

    const age = this.t - this.t0;
    let x = 0;
    let y = Math.sin(this.phase) * c.bob;
    let sq = 0;
    if (this.state === "success" && age < 1) {
      y += Math.abs(Math.sin(age * Math.PI * 2.5)) * 0.5 * (1 - age);
      sq = 0.08 * Math.sin(age * Math.PI * 5) * (1 - age);
    }
    if (this.state === "error" && age < 0.8) x = Math.sin(age * 55) * 0.07 * (1 - age / 0.8);

    const s =
      1 + this.energy * 0.1 + (this.state === "speaking" ? 0.025 * Math.sin(this.t * 9) : 0);
    this.root.position.set(x, y, 0);
    this.root.scale.set(s * (1 + sq), s * (1 - sq), s * (1 + sq));
    this.root.rotation.y = this.lookC.x * 0.4;
    this.root.rotation.x = -this.lookC.y * 0.28;

    if (this.t > this.nextBlink) {
      this.blink = this.t;
      this.nextBlink = this.t + 2 + Math.random() * 3;
    }
    const b = this.t - this.blink;
    const close = b < 0.16 ? Math.sin((b / 0.16) * Math.PI) : 0;
    const open = Math.max(0.04, c.eye * (1 - close * 0.95));
    for (const e of this.eyes) e.scale.y = 0.13 * open;
  }
}
