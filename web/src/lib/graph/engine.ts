/**
 * GraphEngine — the ported three.js graph view core.
 *
 * Faithfully ported from `context/refs/react/graph-react/src/KnowledgeGraph.jsx`
 * (`GraphEngine` 235-1362), which the project owns; the port keeps the reference
 * algorithms and visual behaviour (force/groups/radial layouts, the 2D↔3D morph,
 * CPU screen-space picking, the DOM label layer, camera framing and SVG export)
 * and adds SDT's `hierarchy`/`circular` layouts. It is deliberately React-free:
 * `GraphView.tsx` owns the React lifecycle and the controlled props.
 *
 * Pure logic lives in the sibling modules (`colors`, `layout`, `geometry`,
 * `shaders`, `labels`) so the WebGL class stays thin and is browser-verified.
 */
import * as THREE from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";
import { EDGE_FS, EDGE_VS, NODE_FS, NODE_VS } from "./shaders";
import { EDGE_LABEL_CSS, LABEL_CSS } from "./labels";
import {
  buildColorMaps,
  DEFAULT_BG,
  DEFAULT_GROUP,
  DEFAULT_PALETTE,
  NEUTRAL,
  toRGB,
} from "./colors";
import { bounds, clamp, escapeSvg, fitDistance, smooth } from "./geometry";
import {
  circularPositions,
  computeLinkParams,
  DEFAULT_PHYSICS,
  groupCenters,
  hierarchyPositions,
  radialRings,
  type LayoutLinkLike,
  type LayoutNodeLike,
  type Physics,
} from "./layout";
import type { GraphLinkInput, GraphMode, GraphNodeInput, GraphLayout, LabelMode } from "./types";

export interface EngineEvents {
  onHover?: (node: GraphNodeInput | null) => void;
  onClick?: (node: GraphNodeInput | null) => void;
  onDoubleClick?: (node: GraphNodeInput | null) => void;
  onEdgeHover?: (link: GraphLinkInput | null) => void;
}

export interface EngineStyle {
  glow: number;
  nodeScale: number;
  labels: LabelMode;
  labelMax: number;
  edgeOpacity: number;
  autoRotate: boolean;
}

export type EngineStyleInput = Partial<EngineStyle> & { physics?: Partial<Physics> };

export interface EngineColors {
  nodes?: Record<string, string>;
  relations?: Record<string, string>;
  palette?: string[];
}

interface EngineNode {
  id: string;
  raw: GraphNodeInput;
  index: number;
  group: string;
  deg: number;
}

interface EngineLink {
  a: number;
  b: number;
  raw: GraphLinkInput;
  type?: string;
}

interface CameraGoal {
  type: "fit" | "focus" | "free";
  ttl: number;
  useDir: boolean;
  index: number;
}

const DEFAULT_STYLE: EngineStyle = {
  glow: 0.45,
  nodeScale: 1,
  labels: "auto",
  labelMax: 45,
  edgeOpacity: 0.8,
  autoRotate: false,
};

const ALL_LAYOUTS: GraphLayout[] = ["force", "groups", "radial", "hierarchy", "circular"];

const endId = (x: string | GraphNodeInput | undefined): string | undefined =>
  x !== null && typeof x === "object" ? x.id : (x as string | undefined);

export class GraphEngine {
  host: HTMLElement;
  events: EngineEvents = {};
  opts: EngineStyle = { ...DEFAULT_STYLE };
  p: Physics = { ...DEFAULT_PHYSICS };
  colors: { group: Map<string, string>; rel: Map<string, string> } = {
    group: new Map(),
    rel: new Map(),
  };

  mode: GraphMode = "3d";
  layout: GraphLayout = "force";
  centrality = false;
  neighborhoodId: string | null = null;
  flat = 0;
  flatTarget = 0;
  alpha = 1;
  n = 0;

  nodes: EngineNode[] | null = null;
  links: EngineLink[] = [];
  adj: { o: number; li: number }[][] = [];
  idx = new Map<string, number>();

  hover = -1;
  hoverEdge = -1;
  sel = -1;
  selId: string | null = null;
  pathNodeIds = new Set<string>();
  pathLinks = new Set<GraphLinkInput>();
  hiddenGroups = new Set<string>();
  hiddenRelations = new Set<string>();
  focus = -1;

  w: number;
  h: number;
  pr: number;

  renderer: THREE.WebGLRenderer;
  canvas: HTMLCanvasElement;
  labelLayer: HTMLDivElement;
  scene: THREE.Scene;
  camera: THREE.PerspectiveCamera;
  controls: OrbitControls;
  nodeMat: THREE.ShaderMaterial;
  edgeMat: THREE.ShaderMaterial;
  points: THREE.Points | null = null;
  lines: THREE.LineSegments | null = null;

  pos!: Float32Array;
  vel!: Float32Array;
  order: number[] = [];
  sx!: Float32Array;
  sy!: Float32Array;
  sr!: Float32Array;
  depth!: Float32Array;
  vis!: Uint8Array;
  radius!: Float32Array;
  _ord: number[] = [];
  gIndex!: Int32Array;
  groupCount = 1;
  _cen!: Float32Array;
  lStr!: Float32Array;
  lDist!: Float32Array;
  lBias!: Float32Array;
  ringR: Float32Array | null = null;
  maxDegree = 1;

  posAttr!: THREE.BufferAttribute;
  sizeArr!: Float32Array;
  sizeAttr!: THREE.BufferAttribute;
  colArr!: Float32Array;
  colAttr!: THREE.BufferAttribute;
  alphaArr!: Float32Array;
  alphaTarget!: Float32Array;
  alphaAttr!: THREE.BufferAttribute;
  boostArr!: Float32Array;
  boostTarget!: Float32Array;
  boostAttr!: THREE.BufferAttribute;
  idxArr!: Uint32Array;
  idxAttr!: THREE.BufferAttribute;
  epos!: Float32Array;
  eposAttr!: THREE.BufferAttribute;
  ecolArr!: Float32Array;
  ecolAttr!: THREE.BufferAttribute;
  ealphaArr!: Float32Array;
  ealphaTarget!: Float32Array;
  ealphaAttr!: THREE.BufferAttribute;

  private readonly _rgb = new Map<string, [number, number, number]>();
  private _colorKey = "";
  private _dirty = false;
  private _focusDist = 400;
  private _down: { x: number; y: number; t: number } | null = null;
  private _acc = 0;
  private _fc = 0;
  private _last = 0;
  private _suppressClickUntil = 0;
  private _frame: (now: number) => void;
  private raf = 0;

  private ptr = { x: 0, y: 0, inside: false };
  private goal: CameraGoal = { type: "fit", ttl: 5, useDir: true, index: -1 };
  private dirGoal = new THREE.Vector3(0.5, 0.36, 0.8).normalize();
  private _v = new THREE.Vector3();
  private _off = new THREE.Vector3();
  private _fwd = new THREE.Vector3();
  private _q = new THREE.Quaternion();
  private _qi = new THREE.Quaternion();

  private labelEls = new Map<number, HTMLDivElement>();
  private edgeLabelEls = new Map<number, HTMLDivElement>();
  labelPositions = new Map<number, { centerX: number; top: number; bounds: number[] }>();
  private _shownLabels = new Set<number>();
  private _shownEdgeLabels = new Set<number>();

  constructor(host: HTMLElement) {
    this.host = host;

    const w = (this.w = host.clientWidth || 800);
    const h = (this.h = host.clientHeight || 600);
    this.pr = Math.min(window.devicePixelRatio || 1, 2);

    this.renderer = new THREE.WebGLRenderer({
      antialias: true,
      alpha: true,
      powerPreference: "high-performance",
    });
    this.renderer.setPixelRatio(this.pr);
    this.renderer.setSize(w, h, false);
    this.renderer.setClearColor(0x000000, 0);
    this.canvas = this.renderer.domElement;
    Object.assign(this.canvas.style, {
      position: "absolute",
      inset: "0",
      width: "100%",
      height: "100%",
      display: "block",
      touchAction: "none",
      outline: "none",
      cursor: "grab",
    });
    host.appendChild(this.canvas);

    this.labelLayer = document.createElement("div");
    Object.assign(this.labelLayer.style, {
      position: "absolute",
      inset: "0",
      overflow: "hidden",
      pointerEvents: "none",
    });
    host.appendChild(this.labelLayer);

    this.scene = new THREE.Scene();
    this.camera = new THREE.PerspectiveCamera(50, w / h, 1, 30000);
    this.camera.position.set(0, 0, 420);

    const c = (this.controls = new OrbitControls(this.camera, this.canvas));
    c.enableDamping = true;
    c.dampingFactor = 0.08;
    c.rotateSpeed = 0.7;
    c.zoomSpeed = 0.9;
    c.minDistance = 15;
    c.maxDistance = 15000;
    c.zoomToCursor = true;
    this._applyControlScheme("3d");

    this.nodeMat = new THREE.ShaderMaterial({
      uniforms: {
        uScale: { value: 1 },
        uHalo: { value: 3 },
        uGlow: { value: this.opts.glow },
        uFocus: { value: 400 },
        uDepthFade: { value: 0 },
      },
      vertexShader: NODE_VS,
      fragmentShader: NODE_FS,
      transparent: true,
      depthWrite: false,
      depthTest: false,
    });
    this.edgeMat = new THREE.ShaderMaterial({
      uniforms: {
        uOpacity: { value: this.opts.edgeOpacity },
        uFocus: { value: 400 },
        uDepthFade: { value: 0 },
      },
      vertexShader: EDGE_VS,
      fragmentShader: EDGE_FS,
      transparent: true,
      depthWrite: false,
      depthTest: false,
    });

    this._onMove = (e) => {
      const r = this.canvas.getBoundingClientRect();
      this.ptr.x = e.clientX - r.left;
      this.ptr.y = e.clientY - r.top;
      this.ptr.inside = true;
    };
    this._onLeave = () => {
      this.ptr.inside = false;
      if (this.hover !== -1) this._setHover(-1);
      if (this.hoverEdge !== -1) this._setHoverEdge(-1);
    };
    this._onDown = (e) => {
      const r = this.canvas.getBoundingClientRect();
      this._down = { x: e.clientX - r.left, y: e.clientY - r.top, t: performance.now() };
      this.goal.type = "free";
    };
    this._onUp = (e) => {
      const d = this._down;
      this._down = null;
      if (!d) return;
      const r = this.canvas.getBoundingClientRect();
      const x = e.clientX - r.left;
      const y = e.clientY - r.top;
      if (Math.hypot(x - d.x, y - d.y) < 5 && performance.now() - d.t < 600) {
        if (performance.now() < this._suppressClickUntil) return;
        const hit = this._pickAt(x, y);
        this.events.onClick?.(hit >= 0 ? (this.nodes as EngineNode[])[hit].raw : null);
      }
    };
    this._onCancel = () => {
      this._down = null;
    };
    this._onWheel = () => {
      this.goal.type = "free";
    };
    this._onDbl = (e) => {
      this._suppressClickUntil = performance.now() + 350;
      const r = this.canvas.getBoundingClientRect();
      const hit = this._pickAt(e.clientX - r.left, e.clientY - r.top);
      if (hit >= 0) this.events.onDoubleClick?.((this.nodes as EngineNode[])[hit].raw);
    };
    const cv = this.canvas;
    cv.addEventListener("pointermove", this._onMove);
    cv.addEventListener("pointerleave", this._onLeave);
    cv.addEventListener("pointerdown", this._onDown);
    cv.addEventListener("pointerup", this._onUp);
    cv.addEventListener("pointercancel", this._onCancel);
    cv.addEventListener("wheel", this._onWheel, { passive: true });
    cv.addEventListener("dblclick", this._onDbl);

    this.ro = new ResizeObserver(() => this.resize());
    this.ro.observe(host);

    this._frame = this._loop.bind(this);
    this.raf = requestAnimationFrame(this._frame);
  }

  private ro: ResizeObserver;
  private _onMove: (e: PointerEvent) => void;
  private _onLeave: () => void;
  private _onDown: (e: PointerEvent) => void;
  private _onUp: (e: PointerEvent) => void;
  private _onCancel: () => void;
  private _onWheel: () => void;
  private _onDbl: (e: MouseEvent) => void;

  /* ---------------------------- public API ---------------------------- */

  setStyle(o: EngineStyleInput = {}): void {
    const { physics, ...rest } = o;
    const prevScale = this.opts.nodeScale;
    for (const k of Object.keys(rest) as (keyof EngineStyle)[]) {
      const value = rest[k];
      if (value !== undefined) this.opts[k] = value as never;
    }
    this.nodeMat.uniforms.uGlow.value = this.opts.glow;
    this.edgeMat.uniforms.uOpacity.value = this.opts.edgeOpacity;
    if (this.nodes && prevScale !== this.opts.nodeScale) this._applyRadius();
    if (physics) {
      Object.assign(this.p, physics);
      if (this.nodes) {
        this._computeLayoutParams();
        this.reheat(0.8);
      }
    }
  }

  setColors(maps: EngineColors): void {
    const key = JSON.stringify(maps);
    if (key === this._colorKey) return;
    this._colorKey = key;
    const colorMaps = buildColorMaps(
      (this.nodes ?? []).map((n) => n.raw),
      this.links.map((l) => l.raw),
      maps.nodes ?? {},
      maps.relations ?? {},
      maps.palette ?? DEFAULT_PALETTE,
    );
    this.colors = { group: new Map(colorMaps.groups), rel: new Map(colorMaps.relations) };
    if (this.nodes) this._applyColors();
  }

  setMode(mode: GraphMode): void {
    const m: GraphMode = mode === "2d" ? "2d" : "3d";
    if (m === this.mode) return;
    this.mode = m;
    this.flatTarget = m === "2d" ? 1 : 0;
    this._applyControlScheme(m);
    if (m === "2d") this.dirGoal.set(0, 0, 1);
    else this.dirGoal.set(0.5, 0.36, 0.8).normalize();

    if (!this.nodes) {
      this.flat = this.flatTarget;
      this.camera.position.copy(this.controls.target).addScaledVector(this.dirGoal, 420);
      return;
    }
    if (m === "3d") {
      const P = this.pos;
      for (let i = 0; i < this.n; i += 1) {
        if (Math.abs(P[i * 3 + 2]) < 4) P[i * 3 + 2] = (Math.random() - 0.5) * 40;
      }
    }
    this.reheat(0.9);
    this.goal = { type: "fit", ttl: 4.5, useDir: true, index: -1 };
  }

  setLayout(layout: GraphLayout): void {
    const l: GraphLayout = ALL_LAYOUTS.includes(layout) ? layout : "force";
    if (l === this.layout) return;
    this.layout = l;
    if (!this.nodes) return;
    this._computeLayoutParams();
    this._applyStaticLayout();
    this.goal = { type: "fit", ttl: 4.5, useDir: false, index: -1 };
  }

  setSelected(id: string | null): void {
    this.selId = id ?? null;
    if (!this.nodes) return;
    const idx = id == null ? -1 : (this.idx.get(id) ?? -1);
    if (idx === this.sel) return;
    this.sel = idx;
    this._refreshTargets();
    if (this.layout === "radial") {
      this._computeLayoutParams();
      this.reheat(0.7);
      this.goal = { type: "fit", ttl: 3.5, useDir: false, index: -1 };
    }
  }

  setPath(path: { nodeIds?: string[]; links?: GraphLinkInput[] } | null): void {
    this.pathNodeIds = new Set(path?.nodeIds ?? []);
    this.pathLinks = new Set(path?.links ?? []);
    this._refreshTargets(true);
  }

  setFilters(hiddenGroups: Iterable<string>, hiddenRelations: Iterable<string>): void {
    this.hiddenGroups = new Set(hiddenGroups);
    this.hiddenRelations = new Set(hiddenRelations);
    this._refreshTargets(true);
  }

  setCentrality(enabled: boolean): void {
    this.centrality = Boolean(enabled);
    if (this.nodes) this._applyRadius();
    this._refreshTargets(true);
  }

  setNeighborhood(id: string | null): void {
    this.neighborhoodId = id ?? null;
    this._refreshTargets(true);
  }

  focusNode(id: string): void {
    if (!this.nodes) return;
    const i = this.idx.get(id);
    if (i === undefined) return;
    this.goal = { type: "focus", ttl: 1.8, useDir: false, index: i };
  }

  fitView(): void {
    this.goal = { type: "fit", ttl: 3.5, useDir: false, index: -1 };
  }

  toSVG(): string | null {
    if (!this.nodes) return null;
    this.controls.update();
    this.camera.updateMatrixWorld();
    this._project();
    this._updateLabels();

    const defs = [
      '<radialGradient id="bg" cx="50%" cy="38%" r="75%"><stop offset="0%" stop-color="#101a2e"/><stop offset="58%" stop-color="#080d18"/><stop offset="100%" stop-color="#04070d"/></radialGradient>',
    ];
    const edges: string[] = [];
    for (let li = 0; li < this.links.length; li += 1) {
      const link = this.links[li];
      const a = link.a;
      const b = link.b;
      if (!this.vis[a] || !this.vis[b]) continue;
      const opacity = this.ealphaArr[li * 8] * this.opts.edgeOpacity;
      if (opacity < 0.015) continue;
      const color = this._relColor(link) ?? this._nodeColor((this.nodes as EngineNode[])[a]);
      const marker = `arrow-${li}`;
      defs.push(
        `<marker id="${marker}" markerWidth="9" markerHeight="8" refX="8" refY="4" orient="auto" markerUnits="userSpaceOnUse"><path d="M 0 0 L 8 4 L 0 8 Z" fill="${escapeSvg(color)}"/></marker>`,
      );
      const dx = this.sx[b] - this.sx[a];
      const dy = this.sy[b] - this.sy[a];
      const length = Math.hypot(dx, dy) || 1;
      const ux = dx / length;
      const uy = dy / length;
      const x1 = this.sx[a] + ux * this.sr[a];
      const y1 = this.sy[a] + uy * this.sr[a];
      const x2 = this.sx[b] - ux * this.sr[b];
      const y2 = this.sy[b] - uy * this.sr[b];
      edges.push(
        `<line x1="${x1.toFixed(2)}" y1="${y1.toFixed(2)}" x2="${x2.toFixed(2)}" y2="${y2.toFixed(2)}" stroke="${escapeSvg(color)}" stroke-width="1.4" opacity="${opacity.toFixed(3)}" marker-end="url(#${marker})"/>`,
      );
    }

    const nodes: string[] = [];
    for (let i = 0; i < this.n; i += 1) {
      if (!this.vis[i]) continue;
      const color = escapeSvg(this._nodeColor((this.nodes as EngineNode[])[i]));
      const opacity = this.alphaArr[i];
      const radius = Math.max(3, this.sr[i]);
      const x = this.sx[i].toFixed(2);
      const y = this.sy[i].toFixed(2);
      if (this.boostArr[i] > 0.02) {
        nodes.push(
          `<circle cx="${x}" cy="${y}" r="${(radius * 1.7).toFixed(2)}" fill="${color}" opacity="${(this.boostArr[i] * 0.18).toFixed(3)}"/>`,
        );
      }
      nodes.push(
        `<circle cx="${x}" cy="${y}" r="${radius.toFixed(2)}" fill="${color}" opacity="${opacity.toFixed(3)}"/>`,
      );
      const position = this.labelPositions.get(i);
      if (position) {
        const label = escapeSvg(
          (this.nodes as EngineNode[])[i].raw.label ?? (this.nodes as EngineNode[])[i].id,
        );
        nodes.push(
          `<text x="${position.centerX.toFixed(2)}" y="${(position.top + 13).toFixed(2)}" fill="#e2e8f0" opacity="${opacity.toFixed(3)}" text-anchor="middle" font-family="sans-serif" font-size="11.5">${label}</text>`,
        );
      }
    }

    return `<svg xmlns="http://www.w3.org/2000/svg" width="${this.w}" height="${this.h}" viewBox="0 0 ${this.w} ${this.h}"><defs>${defs.join("")}</defs><rect width="100%" height="100%" fill="url(#bg)"/>${edges.join("")}${nodes.join("")}</svg>`;
  }

  reheat(v: number): void {
    this.alpha = Math.max(this.alpha, v);
  }

  get backgroundColor(): string {
    return DEFAULT_BG;
  }

  setData(rawNodes: GraphNodeInput[] = [], rawLinks: GraphLinkInput[] = []): void {
    if (!Array.isArray(rawNodes) || !Array.isArray(rawLinks)) {
      throw new TypeError("nodes e links devono essere array");
    }
    const prev = new Map<string, [number, number, number]>();
    if (this.nodes) {
      for (const nd of this.nodes) {
        const i = nd.index * 3;
        prev.set(nd.id, [this.pos[i], this.pos[i + 1], this.pos[i + 2]]);
      }
    }

    const seenIds = new Set<string>();
    this.nodes = rawNodes.map((raw, i) => {
      if (!raw || raw.id === undefined || raw.id === null) {
        throw new TypeError(`nodes[${i}] deve avere un id non nullo`);
      }
      if (seenIds.has(raw.id)) throw new TypeError(`id nodo duplicato: ${String(raw.id)}`);
      seenIds.add(raw.id);
      return { id: raw.id, raw, index: i, group: String(raw.group ?? DEFAULT_GROUP), deg: 0 };
    });
    const n = (this.n = this.nodes.length);
    this.idx = new Map();
    this.nodes.forEach((nd) => this.idx.set(nd.id, nd.index));

    this.links = [];
    for (const raw of rawLinks) {
      const a = this.idx.get(endId(raw.source) ?? "");
      const b = this.idx.get(endId(raw.target) ?? "");
      if (a === undefined || b === undefined || a === b) continue;
      this.links.push({ a, b, raw, type: raw.type });
    }
    this.adj = Array.from({ length: n }, () => []);
    this.links.forEach((l, li) => {
      this.nodes![l.a].deg += 1;
      this.nodes![l.b].deg += 1;
      this.adj[l.a].push({ o: l.b, li });
      this.adj[l.b].push({ o: l.a, li });
    });
    this.maxDegree = Math.max(1, ...this.nodes.map((node) => node.deg));

    this.pos = new Float32Array(n * 3);
    this.vel = new Float32Array(n * 3);
    const placed = new Uint8Array(n);
    const spread = 30 + 6 * Math.sqrt(n);
    const flat3 = this.flatTarget === 1 ? 0 : 1;
    this.nodes.forEach((nd, i) => {
      const p = prev.get(nd.id);
      if (p) {
        this.pos.set(p, i * 3);
        placed[i] = 1;
      }
    });
    this.nodes.forEach((_nd, i) => {
      if (placed[i]) return;
      let base: number | null = null;
      for (const e of this.adj[i]) {
        if (placed[e.o]) {
          base = e.o * 3;
          break;
        }
      }
      const j = i * 3;
      if (base !== null) {
        this.pos[j] = this.pos[base] + (Math.random() - 0.5) * 14;
        this.pos[j + 1] = this.pos[base + 1] + (Math.random() - 0.5) * 14;
        this.pos[j + 2] = (this.pos[base + 2] + (Math.random() - 0.5) * 14) * flat3;
      } else {
        this.pos[j] = (Math.random() - 0.5) * 2 * spread;
        this.pos[j + 1] = (Math.random() - 0.5) * 2 * spread;
        this.pos[j + 2] = (Math.random() - 0.5) * 2 * spread * flat3;
      }
      placed[i] = 1;
    });

    this.order = this.nodes
      .map((nd) => nd.index)
      .sort((x, y) => {
        const a = this.nodes![x];
        const b = this.nodes![y];
        return (b.raw.size ?? 0) * 1000 + b.deg - ((a.raw.size ?? 0) * 1000 + a.deg);
      });

    this.sx = new Float32Array(n);
    this.sy = new Float32Array(n);
    this.sr = new Float32Array(n);
    this.depth = new Float32Array(n);
    this.vis = new Uint8Array(n);
    this.radius = new Float32Array(n);
    this._ord = Array.from({ length: n }, (_, i) => i);

    this._clearLabels();
    this._buildGeometry();
    this._applyRadius();
    this._applyColors();
    this.sel = this.selId == null ? -1 : (this.idx.get(this.selId) ?? -1);
    this.hover = -1;
    this._computeLayoutParams();
    this._applyStaticLayout();
    this._refreshTargets(true);
    this._syncEdges();
    this.reheat(1);
    this.goal = { type: "fit", ttl: 5.5, useDir: false, index: -1 };
  }

  resize(): void {
    this.w = this.host.clientWidth || 1;
    this.h = this.host.clientHeight || 1;
    this.renderer.setSize(this.w, this.h, false);
    this.camera.aspect = this.w / this.h;
    this.camera.updateProjectionMatrix();
  }

  dispose(): void {
    cancelAnimationFrame(this.raf);
    this.ro.disconnect();
    const cv = this.canvas;
    cv.removeEventListener("pointermove", this._onMove);
    cv.removeEventListener("pointerleave", this._onLeave);
    cv.removeEventListener("pointerdown", this._onDown);
    cv.removeEventListener("pointerup", this._onUp);
    cv.removeEventListener("pointercancel", this._onCancel);
    cv.removeEventListener("wheel", this._onWheel);
    cv.removeEventListener("dblclick", this._onDbl);
    this.controls.dispose();
    this.points?.geometry.dispose();
    this.lines?.geometry.dispose();
    this.nodeMat.dispose();
    this.edgeMat.dispose();
    this.renderer.dispose();
    cv.remove();
    this.labelLayer.remove();
  }

  /* ----------------------------- construction ---------------------------- */

  private _applyControlScheme(mode: GraphMode): void {
    const c = this.controls;
    if (mode === "2d") {
      c.enableRotate = false;
      c.screenSpacePanning = true;
      c.mouseButtons = { LEFT: THREE.MOUSE.PAN, MIDDLE: THREE.MOUSE.DOLLY, RIGHT: THREE.MOUSE.PAN };
      c.touches = { ONE: THREE.TOUCH.PAN, TWO: THREE.TOUCH.DOLLY_PAN };
    } else {
      c.enableRotate = true;
      c.screenSpacePanning = true;
      c.mouseButtons = {
        LEFT: THREE.MOUSE.ROTATE,
        MIDDLE: THREE.MOUSE.DOLLY,
        RIGHT: THREE.MOUSE.PAN,
      };
      c.touches = { ONE: THREE.TOUCH.ROTATE, TWO: THREE.TOUCH.DOLLY_PAN };
    }
  }

  private _buildGeometry(): void {
    const n = this.n;
    const m = this.links.length;
    if (this.points) {
      this.scene.remove(this.points);
      this.points.geometry.dispose();
    }
    if (this.lines) {
      this.scene.remove(this.lines);
      this.lines.geometry.dispose();
    }

    const dyn = (arr: Float32Array | Uint32Array, size: number) =>
      new THREE.BufferAttribute(arr, size).setUsage(THREE.DynamicDrawUsage);

    const g = new THREE.BufferGeometry();
    this.posAttr = dyn(this.pos, 3);
    this.sizeArr = new Float32Array(n);
    this.sizeAttr = dyn(this.sizeArr, 1);
    this.colArr = new Float32Array(n * 3);
    this.colAttr = dyn(this.colArr, 3);
    this.alphaArr = new Float32Array(n).fill(1);
    this.alphaTarget = new Float32Array(n).fill(1);
    this.alphaAttr = dyn(this.alphaArr, 1);
    this.boostArr = new Float32Array(n);
    this.boostTarget = new Float32Array(n);
    this.boostAttr = dyn(this.boostArr, 1);
    this.idxArr = new Uint32Array(n);
    for (let i = 0; i < n; i += 1) this.idxArr[i] = i;
    this.idxAttr = dyn(this.idxArr, 1);
    g.setAttribute("position", this.posAttr);
    g.setAttribute("aSize", this.sizeAttr);
    g.setAttribute("aColor", this.colAttr);
    g.setAttribute("aAlpha", this.alphaAttr);
    g.setAttribute("aBoost", this.boostAttr);
    g.setIndex(this.idxAttr);
    this.points = new THREE.Points(g, this.nodeMat);
    this.points.frustumCulled = false;
    this.points.renderOrder = 2;
    this.scene.add(this.points);

    const eg = new THREE.BufferGeometry();
    this.epos = new Float32Array(m * 24);
    this.eposAttr = dyn(this.epos, 3);
    this.ecolArr = new Float32Array(m * 24);
    this.ecolAttr = dyn(this.ecolArr, 3);
    this.ealphaArr = new Float32Array(m * 8).fill(0.5);
    this.ealphaTarget = new Float32Array(m * 8).fill(0.5);
    this.ealphaAttr = dyn(this.ealphaArr, 1);
    eg.setAttribute("position", this.eposAttr);
    eg.setAttribute("aColor", this.ecolAttr);
    eg.setAttribute("aAlpha", this.ealphaAttr);
    this.lines = new THREE.LineSegments(eg, this.edgeMat);
    this.lines.frustumCulled = false;
    this.lines.renderOrder = 1;
    this.scene.add(this.lines);
  }

  private _rgbOf(css: string): [number, number, number] {
    let v = this._rgb.get(css);
    if (!v) {
      v = toRGB(css);
      this._rgb.set(css, v);
    }
    return v;
  }

  private _nodeColor(nd: EngineNode): string {
    return nd.raw.color ?? this.colors.group.get(nd.group) ?? NEUTRAL;
  }

  private _relColor(l: EngineLink): string | null {
    if (l.raw.color) return l.raw.color;
    if (l.type !== undefined && l.type !== null) return this.colors.rel.get(String(l.type)) ?? null;
    return null;
  }

  private _applyColors(): void {
    this.nodes!.forEach((nd, i) => {
      const c = this._rgbOf(this._nodeColor(nd));
      this.colArr.set(c, i * 3);
    });
    this.colAttr.needsUpdate = true;
    this.links.forEach((l, li) => {
      const rc = this._relColor(l);
      let c: [number, number, number];
      if (rc) {
        c = this._rgbOf(rc);
      } else {
        const A = this.colArr;
        const a = l.a * 3;
        const b = l.b * 3;
        c = [(A[a] + A[b]) * 0.45, (A[a + 1] + A[b + 1]) * 0.45, (A[a + 2] + A[b + 2]) * 0.45];
      }
      for (let vertex = 0; vertex < 8; vertex += 1) this.ecolArr.set(c, li * 24 + vertex * 3);
    });
    this.ecolAttr.needsUpdate = true;
  }

  private _applyRadius(): void {
    const s = Number.isFinite(this.opts.nodeScale) ? Math.max(0.1, this.opts.nodeScale) : 1;
    const maxDegree = Math.max(1, ...this.nodes!.map((nd) => nd.deg));
    this.nodes!.forEach((nd, i) => {
      const requested = nd.raw.size ?? 0.8 + 0.28 * Math.sqrt(nd.deg);
      const centralityScale = this.centrality ? 0.65 + 1.1 * Math.sqrt(nd.deg / maxDegree) : 1;
      const r =
        3.4 * s * centralityScale * (Number.isFinite(requested) ? Math.max(0.1, requested) : 0.8);
      this.radius[i] = r;
      this.sizeArr[i] = r;
    });
    this.sizeAttr.needsUpdate = true;
  }

  private _computeLayoutParams(): void {
    const n = this.n;
    const p = this.p;
    const layout = this.layout;

    const gmap = new Map<string, number>();
    this.gIndex = new Int32Array(n);
    this.nodes!.forEach((nd, i) => {
      let g = gmap.get(nd.group);
      if (g === undefined) {
        g = gmap.size;
        gmap.set(nd.group, g);
      }
      this.gIndex[i] = g;
    });
    this.groupCount = Math.max(1, gmap.size);
    this._cen = new Float32Array(this.groupCount * 3);

    const linkLikes: LayoutLinkLike[] = this.links.map((l) => ({
      a: l.a,
      b: l.b,
      type: l.type,
      weight: l.raw.weight,
    }));
    const nodeLikes: LayoutNodeLike[] = this.nodes!.map((nd) => ({
      id: nd.id,
      group: nd.group,
      deg: nd.deg,
      index: nd.index,
    }));
    const params = computeLinkParams(layout, nodeLikes, linkLikes, this.gIndex, p.linkDistance);
    this.lStr = params.lStr;
    this.lDist = params.lDist;
    this.lBias = params.lBias;

    this.ringR = null;
    if (layout === "radial" && n) {
      this.ringR = radialRings(this.adj, n, this.sel, this.order, p.ringGap);
    }
  }

  /** Position a deterministic layout and freeze the simulation. */
  private _applyStaticLayout(): void {
    if (!this.nodes) return;
    if (this.layout !== "hierarchy" && this.layout !== "circular") {
      this.reheat(0.9);
      return;
    }
    const nodeLikes: LayoutNodeLike[] = this.nodes.map((nd) => ({
      id: nd.id,
      group: nd.group,
      deg: nd.deg,
      index: nd.index,
    }));
    const linkLikes: LayoutLinkLike[] = this.links.map((l) => ({
      a: l.a,
      b: l.b,
      type: l.type,
      weight: l.raw.weight,
    }));
    const pos =
      this.layout === "hierarchy"
        ? hierarchyPositions(nodeLikes, linkLikes)
        : circularPositions(nodeLikes);
    this.pos.set(pos);
    this.vel.fill(0);
    this.alpha = 0;
    this._syncEdges();
  }

  /* ------------------------ highlight / state ------------------------ */

  private _setHover(i: number): void {
    this.hover = i;
    this._refreshTargets();
    this.canvas.style.cursor = i >= 0 ? "pointer" : "grab";
    this.events.onHover?.(i >= 0 ? this.nodes![i].raw : null);
  }

  private _setHoverEdge(i: number): void {
    if (i === this.hoverEdge) return;
    this.hoverEdge = i;
    if (this.hover < 0) this.canvas.style.cursor = i >= 0 ? "pointer" : "grab";
    this.events.onEdgeHover?.(i >= 0 ? this.links[i].raw : null);
  }

  private _refreshTargets(instant = false): void {
    if (!this.nodes) return;
    const n = this.n;
    const f = this.hover >= 0 ? this.hover : this.sel;
    this.focus = f;
    if (this.pathNodeIds.size > 0) {
      for (let i = 0; i < n; i += 1) {
        const onPath = this.pathNodeIds.has(this.nodes[i].id);
        this.alphaTarget[i] = onPath ? 1 : 0.08;
        this.boostTarget[i] = onPath ? 0.85 : i === this.hover || i === this.sel ? 0.25 : 0;
      }
      this.links.forEach((l, li) => {
        const opacity = this.pathLinks.has(l.raw) ? 1 : 0.035;
        this.ealphaTarget.fill(opacity, li * 8, li * 8 + 8);
      });
    } else if (f < 0) {
      this.alphaTarget.fill(1);
      this.nodes.forEach((nd, i) => {
        this.boostTarget[i] = this.centrality
          ? 0.12 + 0.62 * Math.sqrt(nd.deg / Math.max(1, this.maxDegree))
          : 0;
      });
      this.ealphaTarget.fill(0.5);
    } else {
      const active = new Uint8Array(n);
      active[f] = 1;
      for (const e of this.adj[f]) active[e.o] = 2;
      for (let i = 0; i < n; i += 1) {
        this.alphaTarget[i] = active[i] ? 1 : 0.12;
        this.boostTarget[i] = i === this.hover || i === this.sel ? 1 : active[i] === 2 ? 0.35 : 0;
      }
      this.links.forEach((l, li) => {
        const v = l.a === f || l.b === f ? 1 : 0.07;
        this.ealphaTarget.fill(v, li * 8, li * 8 + 8);
      });
    }
    if (this.neighborhoodId != null) {
      const center = this.idx.get(this.neighborhoodId);
      if (center !== undefined) {
        const nearby = new Uint8Array(n);
        nearby[center] = 1;
        for (const edge of this.adj[center]) nearby[edge.o] = 1;
        for (let i = 0; i < n; i += 1) {
          if (!nearby[i] && !this.pathNodeIds.has(this.nodes[i].id)) {
            this.alphaTarget[i] = Math.min(this.alphaTarget[i], 0.035);
            this.boostTarget[i] = 0;
          }
        }
        this.links.forEach((link, li) => {
          const active = link.a === center || link.b === center || this.pathLinks.has(link.raw);
          if (!active) this.ealphaTarget.fill(0.018, li * 8, li * 8 + 8);
        });
      }
    }
    for (let i = 0; i < n; i += 1) {
      if (this.hiddenGroups.has(this.nodes[i].group)) {
        this.alphaTarget[i] = Math.min(this.alphaTarget[i], 0.025);
        this.boostTarget[i] = 0;
      }
    }
    this.links.forEach((link, li) => {
      const hidden =
        this.hiddenRelations.has(String(link.type ?? "")) ||
        this.hiddenGroups.has(this.nodes![link.a].group) ||
        this.hiddenGroups.has(this.nodes![link.b].group);
      if (hidden) this.ealphaTarget.fill(0.012, li * 8, li * 8 + 8);
    });
    if (instant) {
      this.alphaArr.set(this.alphaTarget);
      this.boostArr.set(this.boostTarget);
      this.ealphaArr.set(this.ealphaTarget);
      this.alphaAttr.needsUpdate = true;
      this.boostAttr.needsUpdate = true;
      this.ealphaAttr.needsUpdate = true;
    }
  }

  private _lerpArr(arr: Float32Array, tgt: Float32Array, k: number): boolean {
    let ch = false;
    for (let i = 0; i < arr.length; i += 1) {
      const d = tgt[i] - arr[i];
      if (d > 0.002 || d < -0.002) {
        arr[i] += d * k;
        ch = true;
      } else if (d !== 0) {
        arr[i] = tgt[i];
        ch = true;
      }
    }
    return ch;
  }

  /* ------------------------------ simulation ---------------------------- */

  private _tick(): void {
    const n = this.n;
    if (!n) return;
    const P = this.pos;
    const V = this.vel;
    const a = this.alpha;
    const p = this.p;
    const layout = this.layout;
    const flat = this.flat;
    const maxD2 = p.maxDist * p.maxDist;
    const charge = p.charge * a;

    for (let i = 0; i < n; i += 1) {
      const ix = i * 3;
      const xi = P[ix];
      const yi = P[ix + 1];
      const zi = P[ix + 2];
      let fx = 0;
      let fy = 0;
      let fz = 0;
      for (let j = i + 1; j < n; j += 1) {
        const jx = j * 3;
        let dx = P[jx] - xi;
        let dy = P[jx + 1] - yi;
        let dz = P[jx + 2] - zi;
        let d2 = dx * dx + dy * dy + dz * dz;
        if (d2 > maxD2) continue;
        if (d2 < 1) {
          dx = Math.random() - 0.5;
          dy = Math.random() - 0.5;
          dz = (Math.random() - 0.5) * (1 - flat);
          d2 = dx * dx + dy * dy + dz * dz + 0.01;
        }
        const w = charge / d2;
        fx -= dx * w;
        fy -= dy * w;
        fz -= dz * w;
        V[jx] += dx * w;
        V[jx + 1] += dy * w;
        V[jx + 2] += dz * w;
      }
      V[ix] += fx;
      V[ix + 1] += fy;
      V[ix + 2] += fz;
    }

    const L = this.links;
    for (let li = 0; li < L.length; li += 1) {
      const l = L[li];
      const A = l.a * 3;
      const B = l.b * 3;
      let dx = P[B] - P[A];
      let dy = P[B + 1] - P[A + 1];
      let dz = P[B + 2] - P[A + 2];
      const d = Math.sqrt(dx * dx + dy * dy + dz * dz) || 1e-6;
      const k = ((d - this.lDist[li]) / d) * a * this.lStr[li];
      dx *= k;
      dy *= k;
      dz *= k;
      const b = this.lBias[li];
      V[B] -= dx * b;
      V[B + 1] -= dy * b;
      V[B + 2] -= dz * b;
      V[A] += dx * (1 - b);
      V[A + 1] += dy * (1 - b);
      V[A + 2] += dz * (1 - b);
    }

    if (layout === "groups") this._cen = groupCenters(this.groupCount, n, this.flat);
    for (let i = 0; i < n; i += 1) {
      const ix = i * 3;
      if (layout === "force") {
        const g = p.gravity * a;
        V[ix] -= P[ix] * g;
        V[ix + 1] -= P[ix + 1] * g;
        V[ix + 2] -= P[ix + 2] * g;
      } else if (layout === "groups") {
        const c = this.gIndex[i] * 3;
        const k = 0.14 * a;
        const g = 0.004 * a;
        V[ix] += (this._cen[c] - P[ix]) * k - P[ix] * g;
        V[ix + 1] += (this._cen[c + 1] - P[ix + 1]) * k - P[ix + 1] * g;
        V[ix + 2] += (this._cen[c + 2] - P[ix + 2]) * k - P[ix + 2] * g;
      } else if (layout === "radial" && this.ringR) {
        const x = P[ix];
        const y = P[ix + 1];
        const z = P[ix + 2];
        const r = Math.sqrt(x * x + y * y + z * z);
        if (r < 1e-3) {
          V[ix] += (Math.random() - 0.5) * 2;
          V[ix + 1] += (Math.random() - 0.5) * 2;
          continue;
        }
        const f = ((this.ringR[i] - r) / r) * 0.2 * a;
        V[ix] += x * f;
        V[ix + 1] += y * f;
        V[ix + 2] += z * f;
      }
    }

    if (flat > 0) {
      for (let i = 0; i < n; i += 1) {
        const z = i * 3 + 2;
        V[z] = V[z] * (1 - 0.5 * flat) - P[z] * 0.1 * flat;
      }
    }

    for (let i = 0; i < n * 3; i += 1) {
      V[i] *= 0.6;
      P[i] += V[i];
    }
    if (flat > 0.999 && this.flatTarget === 1) {
      for (let i = 0; i < n; i += 1) {
        P[i * 3 + 2] = 0;
        V[i * 3 + 2] = 0;
      }
    }
    this.alpha *= p.alphaDecay;
    this._dirty = true;
  }

  private _syncEdges(): void {
    const P = this.pos;
    const E = this.epos;
    for (let li = 0; li < this.links.length; li += 1) {
      const l = this.links[li];
      const a = l.a * 3;
      const b = l.b * 3;
      const o = li * 24;
      let dx = P[b] - P[a];
      let dy = P[b + 1] - P[a + 1];
      let dz = P[b + 2] - P[a + 2];
      const distance = Math.sqrt(dx * dx + dy * dy + dz * dz) || 1;
      dx /= distance;
      dy /= distance;
      dz /= distance;

      const tipX = P[b] - dx * (this.radius[l.b] + 1);
      const tipY = P[b + 1] - dy * (this.radius[l.b] + 1);
      const tipZ = P[b + 2] - dz * (this.radius[l.b] + 1);
      const headLength = Math.min(12, Math.max(6, this.radius[l.b] * 1.65));
      const headWidth = headLength * 0.48;
      let px = -dy;
      let py = dx;
      const pz = 0;
      const perpLength = Math.sqrt(px * px + py * py + pz * pz);
      if (perpLength < 0.001) {
        px = 1;
        py = 0;
      } else {
        px /= perpLength;
        py /= perpLength;
      }
      const baseX = tipX - dx * headLength;
      const baseY = tipY - dy * headLength;
      const baseZ = tipZ - dz * headLength;

      E[o] = P[a];
      E[o + 1] = P[a + 1];
      E[o + 2] = P[a + 2];
      E[o + 3] = baseX;
      E[o + 4] = baseY;
      E[o + 5] = baseZ;
      E[o + 6] = baseX + px * headWidth;
      E[o + 7] = baseY + py * headWidth;
      E[o + 8] = baseZ + pz * headWidth;
      E[o + 9] = tipX;
      E[o + 10] = tipY;
      E[o + 11] = tipZ;
      E[o + 12] = tipX;
      E[o + 13] = tipY;
      E[o + 14] = tipZ;
      E[o + 15] = baseX - px * headWidth;
      E[o + 16] = baseY - py * headWidth;
      E[o + 17] = baseZ - pz * headWidth;
      E[o + 18] = baseX - px * headWidth;
      E[o + 19] = baseY - py * headWidth;
      E[o + 20] = baseZ - pz * headWidth;
      E[o + 21] = baseX + px * headWidth;
      E[o + 22] = baseY + py * headWidth;
      E[o + 23] = baseZ + pz * headWidth;
    }
    this.eposAttr.needsUpdate = true;
    this.posAttr.needsUpdate = true;
  }

  /* -------------------------------- camera ------------------------------- */

  private _updateCamera(dt: number): void {
    const g = this.goal;
    if (g.type === "free" || !this.n) return;
    g.ttl -= dt;
    if (g.ttl <= 0) {
      g.type = "free";
      return;
    }
    const cam = this.camera;
    const ctl = this.controls;
    const k = 1 - Math.exp(-dt * 4.5);
    const off = this._off.copy(cam.position).sub(ctl.target);
    let len = off.length() || 1;
    off.divideScalar(len);

    let tx: number;
    let ty: number;
    let tz: number;
    let dist: number;
    if (g.type === "focus" && g.index >= 0 && g.index < this.n) {
      const i = g.index * 3;
      tx = this.pos[i];
      ty = this.pos[i + 1];
      tz = this.pos[i + 2];
      dist = Math.min(len, 170);
    } else {
      const b = bounds(this.pos, this.n);
      tx = b.cx;
      ty = b.cy;
      tz = b.cz;
      dist = fitDistance(b.r, this.camera.fov, this.camera.aspect, this.flat);
    }
    ctl.target.x += (tx - ctl.target.x) * k;
    ctl.target.y += (ty - ctl.target.y) * k;
    ctl.target.z += (tz - ctl.target.z) * k;

    if (g.useDir) {
      this._q.setFromUnitVectors(off, this.dirGoal);
      this._qi.identity().slerp(this._q, k);
      off.applyQuaternion(this._qi);
    }
    len += (dist - len) * k;
    cam.position.copy(ctl.target).addScaledVector(off, len);
  }

  /* ---------------------- projection, picking, labels -------------------- */

  private _project(): void {
    const n = this.n;
    const P = this.pos;
    const cam = this.camera;
    const v = this._v;
    const tanHalf = Math.tan(((cam.fov / 2) * Math.PI) / 180);
    const kPx = this.h / (2 * tanHalf);
    const fwd = cam.getWorldDirection(this._fwd);
    const cp = cam.position;
    for (let i = 0; i < n; i += 1) {
      const ix = i * 3;
      v.set(P[ix], P[ix + 1], P[ix + 2]).project(cam);
      const x = (v.x * 0.5 + 0.5) * this.w;
      const y = (-v.y * 0.5 + 0.5) * this.h;
      this.sx[i] = x;
      this.sy[i] = y;
      const d = (P[ix] - cp.x) * fwd.x + (P[ix + 1] - cp.y) * fwd.y + (P[ix + 2] - cp.z) * fwd.z;
      this.depth[i] = d;
      this.sr[i] = (this.radius[i] * kPx) / Math.max(d, 0.001);
      this.vis[i] = d > 1 && x > -30 && x < this.w + 30 && y > -30 && y < this.h + 30 ? 1 : 0;
    }
  }

  private _pickAt(x: number, y: number): number {
    let best = -1;
    let bestD = Infinity;
    for (let i = 0; i < this.n; i += 1) {
      if (!this.vis[i]) continue;
      const dx = this.sx[i] - x;
      const dy = this.sy[i] - y;
      const rr = Math.max(this.sr[i] * 1.25, 7);
      if (dx * dx + dy * dy <= rr * rr && this.depth[i] < bestD) {
        best = i;
        bestD = this.depth[i];
      }
    }
    return best;
  }

  private _pickLinkAt(x: number, y: number): number {
    let best = -1;
    let bestD2 = 64;
    for (let li = 0; li < this.links.length; li += 1) {
      const { a, b } = this.links[li];
      if (!this.vis[a] || !this.vis[b]) continue;
      const ax = this.sx[a];
      const ay = this.sy[a];
      const dx = this.sx[b] - ax;
      const dy = this.sy[b] - ay;
      const length2 = dx * dx + dy * dy;
      const t = length2 > 0 ? clamp(((x - ax) * dx + (y - ay) * dy) / length2, 0, 1) : 0;
      const ex = x - (ax + t * dx);
      const ey = y - (ay + t * dy);
      const distance2 = ex * ex + ey * ey;
      if (distance2 < bestD2) {
        best = li;
        bestD2 = distance2;
      }
    }
    return best;
  }

  private _sortNodes(): void {
    const n = this.n;
    if (n < 2) return;
    const d = this.depth;
    this._ord.sort((a, b) => d[b] - d[a]);
    for (let i = 0; i < n; i += 1) this.idxArr[i] = this._ord[i];
    this.idxAttr.needsUpdate = true;
  }

  private _clearLabels(): void {
    for (const el of this.labelEls.values()) el.remove();
    for (const el of this.edgeLabelEls.values()) el.remove();
    this.labelEls.clear();
    this.edgeLabelEls.clear();
    this._shownLabels = new Set();
    this._shownEdgeLabels = new Set();
  }

  private _depthFade(d: number): number {
    return (
      1 -
      this.nodeMat.uniforms.uDepthFade.value *
        0.6 *
        smooth(this._focusDist * 0.9, this._focusDist * 1.9, d)
    );
  }

  private _updateLabels(): void {
    const mode = this.opts.labels;
    const shown = new Set<number>();
    const rects: number[][] = [];
    const labelPositions = new Map<number, { centerX: number; top: number; bounds: number[] }>();
    const f = this.focus;

    const place = (i: number, force: boolean): boolean => {
      if (!this.vis[i]) return false;
      const text = String(this.nodes![i].raw.label ?? this.nodes![i].id);
      const wpx = text.length * 6.3 + 10;
      const x = this.sx[i];
      const y = this.sy[i];
      const radius = this.sr[i];
      const candidates: [number, number][] = [
        [x, y + radius + 3],
        [x, y - radius - 19],
        [x + radius + wpx / 2 + 5, y - 8],
        [x - radius - wpx / 2 - 5, y - 8],
        [x + radius + wpx / 2 + 5, y + radius - 8],
        [x - radius - wpx / 2 - 5, y + radius - 8],
        [x + radius + wpx / 2 + 5, y - radius - 8],
        [x - radius - wpx / 2 - 5, y - radius - 8],
      ];
      let position: { centerX: number; top: number; bounds: number[] } | null = null;
      for (const [centerX, top] of candidates) {
        const bnd = [centerX - wpx / 2, top, centerX + wpx / 2, top + 16];
        if (bnd[0] < 0 || bnd[1] < 0 || bnd[2] > this.w || bnd[3] > this.h) continue;
        const overlaps = rects.some(
          (other) =>
            bnd[0] < other[2] + 2 &&
            bnd[2] + 2 > other[0] &&
            bnd[1] < other[3] + 2 &&
            bnd[3] + 2 > other[1],
        );
        if (!overlaps) {
          position = { centerX, top, bounds: bnd };
          break;
        }
      }
      if (!position) {
        if (!force) return false;
        const [centerX, top] = candidates[0];
        position = { centerX, top, bounds: [centerX - wpx / 2, top, centerX + wpx / 2, top + 16] };
      }
      rects.push(position.bounds);
      labelPositions.set(i, position);
      shown.add(i);
      return true;
    };

    if (mode !== "none" && this.n) {
      let pathCount = 0;
      for (const id of this.pathNodeIds) {
        if (pathCount++ >= 18) break;
        const index = this.idx.get(id);
        if (index !== undefined) place(index, false);
      }
      if (f >= 0) {
        place(f, true);
        let c = 0;
        for (const e of this.adj[f]) {
          if (c++ > 60) break;
          place(e.o, false);
        }
        if (this.sel >= 0) place(this.sel, true);
      } else if (mode !== "hover") {
        const cap = mode === "always" ? 400 : this.opts.labelMax;
        for (const i of this.order) {
          if (shown.size >= cap) break;
          place(i, false);
        }
      }
    }

    for (const i of this._shownLabels) {
      if (!shown.has(i)) {
        const el = this.labelEls.get(i);
        if (el) el.style.display = "none";
      }
    }
    for (const i of shown) {
      let el = this.labelEls.get(i);
      if (!el) {
        el = document.createElement("div");
        el.style.cssText = LABEL_CSS;
        el.textContent = String(this.nodes![i].raw.label ?? this.nodes![i].id);
        this.labelLayer.appendChild(el);
        this.labelEls.set(i, el);
      }
      const emph = i === this.hover || i === this.sel;
      const position = labelPositions.get(i)!;
      el.style.transform = `translate(${position.centerX.toFixed(1)}px,${position.top.toFixed(1)}px) translateX(-50%)`;
      el.style.display = "block";
      const labelOpacity = clamp(this._depthFade(this.depth[i]), 0.25, 1);
      el.style.opacity = String(
        this.hiddenGroups.has(this.nodes![i].group) ? Math.min(labelOpacity, 0.16) : labelOpacity,
      );
      el.style.fontWeight = emph ? "600" : "400";
      el.style.color = emph ? "#fff" : "rgba(226,232,240,.92)";
    }
    this.labelPositions = labelPositions;
    this._shownLabels = shown;

    const shownE = new Set<number>();
    if (f >= 0 && mode !== "none") {
      let c = 0;
      for (const e of this.adj[f]) {
        if (c >= 14) break;
        const l = this.links[e.li];
        const text = l.raw.label ?? l.type;
        if (text === undefined || text === null || text === "") continue;
        if (!this.vis[l.a] || !this.vis[l.b]) continue;
        c += 1;
        let el = this.edgeLabelEls.get(e.li);
        if (!el) {
          el = document.createElement("div");
          el.style.cssText = EDGE_LABEL_CSS;
          el.textContent = String(text);
          el.style.color = this._relColor(l) ?? "#cbd5e1";
          this.labelLayer.appendChild(el);
          this.edgeLabelEls.set(e.li, el);
        }
        const mx = (this.sx[l.a] + this.sx[l.b]) / 2;
        const my = (this.sy[l.a] + this.sy[l.b]) / 2;
        el.style.transform = `translate(${mx.toFixed(1)}px,${my.toFixed(1)}px) translate(-50%,-50%)`;
        el.style.display = "block";
        const filtered =
          this.hiddenRelations.has(String(l.type ?? "")) ||
          this.hiddenGroups.has(this.nodes![l.a].group) ||
          this.hiddenGroups.has(this.nodes![l.b].group);
        el.style.opacity = filtered ? "0.12" : "1";
        shownE.add(e.li);
      }
    }
    for (const li of this._shownEdgeLabels) {
      if (!shownE.has(li)) {
        const el = this.edgeLabelEls.get(li);
        if (el) el.style.display = "none";
      }
    }
    this._shownEdgeLabels = shownE;
  }

  /* --------------------------------- loop -------------------------------- */

  private _loop(now: number): void {
    this.raf = requestAnimationFrame(this._frame);
    const dt = clamp((now - (this._last || now)) / 1000, 0.001, 0.05);
    this._last = now;
    if (!this.nodes) {
      this.renderer.render(this.scene, this.camera);
      return;
    }

    const diff = this.flatTarget - this.flat;
    if (Math.abs(diff) > 0.0005) this.flat += diff * (1 - Math.exp(-dt * 3));
    else this.flat = this.flatTarget;

    this._acc += dt;
    let steps = 0;
    while (this._acc >= 1 / 60 && steps < 3) {
      this._acc -= 1 / 60;
      steps += 1;
      if (this.alpha > this.p.alphaMin) this._tick();
    }
    if (this._acc > 1 / 60) this._acc = 0;
    if (this._dirty) {
      this._syncEdges();
      this._dirty = false;
    }

    if (this._lerpArr(this.alphaArr, this.alphaTarget, 1 - Math.exp(-dt * 12))) {
      this.alphaAttr.needsUpdate = true;
    }
    if (this._lerpArr(this.boostArr, this.boostTarget, 1 - Math.exp(-dt * 12))) {
      this.boostAttr.needsUpdate = true;
    }
    if (this._lerpArr(this.ealphaArr, this.ealphaTarget, 1 - Math.exp(-dt * 12))) {
      this.ealphaAttr.needsUpdate = true;
    }

    this._updateCamera(dt);
    this.controls.autoRotate =
      this.opts.autoRotate && this.flatTarget === 0 && this.goal.type === "free";
    this.controls.autoRotateSpeed = 0.6;
    this.controls.update();
    this.camera.updateMatrixWorld();

    const half = ((this.camera.fov / 2) * Math.PI) / 180;
    this._focusDist = this.camera.position.distanceTo(this.controls.target);
    const nu = this.nodeMat.uniforms;
    const eu = this.edgeMat.uniforms;
    nu.uScale.value = (this.h * this.pr) / Math.tan(half);
    nu.uFocus.value = this._focusDist;
    eu.uFocus.value = this._focusDist;
    nu.uDepthFade.value = 1 - this.flat;
    eu.uDepthFade.value = 1 - this.flat;

    this._project();
    if (this.flat < 0.98 && (this._fc++ & 3) === 0) this._sortNodes();

    if (this.ptr.inside && !this._down) {
      const hit = this._pickAt(this.ptr.x, this.ptr.y);
      if (hit !== this.hover) this._setHover(hit);
      const edgeHit = hit < 0 ? this._pickLinkAt(this.ptr.x, this.ptr.y) : -1;
      if (edgeHit !== this.hoverEdge) this._setHoverEdge(edgeHit);
    }
    this._updateLabels();
    this.renderer.render(this.scene, this.camera);
  }
}
