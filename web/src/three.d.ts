/** Minimal ambient types for the three.js classes used by the 3D graph sprites. */
declare module "three" {
  export const SRGBColorSpace: string;
  export const LinearFilter: number;

  export class Vector2 {
    constructor(x: number, y: number);
  }

  export class Object3D {
    scale: { set(x: number, y: number, z: number): void };
  }

  export class Sprite extends Object3D {
    constructor(material?: SpriteMaterial);
  }

  export class SpriteMaterial {
    constructor(params?: Record<string, unknown>);
  }

  export class CanvasTexture {
    colorSpace: string;
    minFilter: number;
    generateMipmaps: boolean;
    constructor(canvas: HTMLCanvasElement);
  }
}

declare module "three/examples/jsm/postprocessing/UnrealBloomPass.js" {
  export class UnrealBloomPass {
    constructor(
      resolution: { x: number; y: number },
      strength?: number,
      radius?: number,
      threshold?: number,
    );
  }
}
