import * as THREE from "three";
import type { GNode } from "./graphModel";
import type { NodeVisual } from "./graphSelection";

/** Base world-space size of a label sprite before node-radius scaling. */
export const SPRITE_BASE = { width: 48, height: 26 };

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
  canvas.width = 512;
  canvas.height = 256;
  const ctx = canvas.getContext("2d");
  if (!ctx) return new THREE.Object3D();

  const dotScale = 4; // 512px canvas, dot radius ~12px in sprite units
  ctx.globalAlpha = spec.alpha;
  ctx.beginPath();
  ctx.arc(160, 64, Math.max(6, spec.radius * dotScale), 0, Math.PI * 2);
  ctx.fillStyle = spec.dotColor;
  ctx.fill();

  if (spec.show) {
    ctx.font = "600 44px 'IBM Plex Sans', sans-serif";
    ctx.textAlign = "center";
    ctx.fillStyle = spec.labelColor;
    ctx.fillText(spec.title, 160, 200);
  }

  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.minFilter = THREE.LinearFilter;
  texture.generateMipmaps = false;
  const material = new THREE.SpriteMaterial({
    map: texture,
    transparent: true,
    alphaTest: 0.5,
    depthWrite: false,
  });
  const sprite = new THREE.Sprite(material);
  const nodeSize = Math.max(spec.radius, 2.4);
  sprite.scale.set((SPRITE_BASE.width / 2.4) * nodeSize, (SPRITE_BASE.height / 2.4) * nodeSize, 1);
  return sprite;
}