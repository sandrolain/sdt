import * as THREE from "three";
import type { GNode } from "./graphModel";
import type { NodeVisual } from "./graphSelection";

/** Base world-space size of a label sprite before node-radius scaling (2:1, as the canvas). */
export const SPRITE_BASE = { width: 48, height: 24 };

/** Sprite canvas: the dot sits at the centre so it matches the node origin (link anchor). */
export const SPRITE_CANVAS = { width: 1024, height: 512 };
const DOT_SCALE = 8; // canvas px per radius unit
const LABEL_FONT = 56;
const LABEL_MIN_FONT = 28;
const LABEL_GAP = 80;

/** Font size for a label so it fits the canvas width without clipping. */
export function labelFontSize(text: string, base = LABEL_FONT, min = LABEL_MIN_FONT): number {
  if (text.length === 0) return base;
  const max = SPRITE_CANVAS.width * 0.94;
  const approx = text.length * base * 0.55; // rough advance width per glyph
  if (approx <= max) return base;
  return Math.max(min, Math.floor((base * max) / approx));
}

/** Node-only glow (baked into the sprite dot): halo radius vs dot, and intensity. */
export const GLOW_SCALE = 3.2;
export const GLOW_ALPHA = 0.3;

/** `rgba()` for a `#rrggbb` colour with the given alpha; passthrough otherwise. */
export function glowColor(hex: string, alpha: number): string {
  const match = /^#([0-9a-f]{6})$/i.exec(hex.trim());
  if (!match) return hex;
  const value = Number.parseInt(match[1], 16);
  const r = (value >> 16) & 0xff;
  const g = (value >> 8) & 0xff;
  const b = value & 0xff;
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

export interface LabelSpriteSpec {
  /** whether a persistent label should be drawn (honors showLabels + highlight) */
  show: boolean;
  title: string;
  dotColor: string;
  labelColor: string;
  alpha: number;
  radius: number;
}

export const NODE_LABEL_COLOR = "#cdd6f4";

/**
 * Pure decision behind the 3D label sprite. Mirrors the 2D renderer: labels are
 * drawn only when `showLabels` is on AND the node is not dimmed by an active
 * selection highlight.
 */
export function labelSpriteSpec(
  node: GNode,
  visual: NodeVisual,
  showLabels: boolean,
): LabelSpriteSpec {
  const radius = Math.sqrt(Math.max(1, node.val ?? 1)) * 2.4;
  return {
    show: showLabels && (visual.emphasis || visual.alpha === 1),
    title: node.title,
    dotColor: node.color ?? "#9399b2",
    labelColor: NODE_LABEL_COLOR,
    alpha: visual.alpha,
    radius,
  };
}

/** Draw the label sprite canvas and return a `THREE.Sprite` positioned at origin. */
export function labelObject(spec: LabelSpriteSpec): THREE.Object3D {
  const canvas = document.createElement("canvas");
  canvas.width = SPRITE_CANVAS.width;
  canvas.height = SPRITE_CANVAS.height;
  const ctx = canvas.getContext("2d");
  if (!ctx) return new THREE.Object3D();

  const cx = SPRITE_CANVAS.width / 2;
  const cy = SPRITE_CANVAS.height / 2;
  const dotR = Math.max(12, spec.radius * DOT_SCALE);
  ctx.globalAlpha = spec.alpha;

  // node-only glow: a soft halo behind the dot (labels never glow)
  const glowR = dotR * GLOW_SCALE;
  const halo = ctx.createRadialGradient(cx, cy, dotR * 0.6, cx, cy, glowR);
  halo.addColorStop(0, glowColor(spec.dotColor, GLOW_ALPHA));
  halo.addColorStop(1, glowColor(spec.dotColor, 0));
  ctx.beginPath();
  ctx.arc(cx, cy, glowR, 0, Math.PI * 2);
  ctx.fillStyle = halo;
  ctx.fill();

  // dot centered on the canvas centre = node origin, so links stay attached
  ctx.beginPath();
  ctx.arc(cx, cy, dotR, 0, Math.PI * 2);
  ctx.fillStyle = spec.dotColor;
  ctx.fill();

  if (spec.show) {
    const size = labelFontSize(spec.title);
    ctx.font = `500 ${size}px 'IBM Plex Sans', sans-serif`;
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillStyle = spec.labelColor;
    ctx.fillText(spec.title, cx, cy + dotR + LABEL_GAP);
  }

  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.minFilter = THREE.LinearFilter;
  texture.generateMipmaps = false;
  const material = new THREE.SpriteMaterial({
    map: texture,
    transparent: true,
    alphaTest: 0.01,
    depthWrite: false,
  });
  const sprite = new THREE.Sprite(material);
  const nodeSize = Math.max(spec.radius, 2.4);
  sprite.scale.set((SPRITE_BASE.width / 2.4) * nodeSize, (SPRITE_BASE.height / 2.4) * nodeSize, 1);
  return sprite;
}