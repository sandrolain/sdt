/**
 * Minimal ambient types for the three.js surface used by the viewer.
 *
 * The project deliberately does not depend on `@types/three`; this stub declares
 * only the members the 3D graph and the ported graph engine touch. Extend it
 * when new three.js members are adopted.
 */
declare module "three" {
  export const SRGBColorSpace: string;
  export const LinearFilter: number;
  export const DynamicDrawUsage: number;

  export const MOUSE: { PAN: number; DOLLY: number; ROTATE: number };
  export const TOUCH: { PAN: number; DOLLY_PAN: number; ROTATE: number };

  export const MathUtils: {
    degToRad(value: number): number;
  };

  export class Vector2 {
    constructor(x: number, y: number);
  }

  export class Vector3 {
    x: number;
    y: number;
    z: number;
    constructor(x?: number, y?: number, z?: number);
    set(x: number, y: number, z: number): this;
    copy(v: Vector3): this;
    clone(): Vector3;
    sub(v: Vector3): this;
    length(): number;
    divideScalar(s: number): this;
    normalize(): this;
    addScaledVector(v: Vector3, s: number): this;
    distanceTo(v: Vector3): number;
    applyQuaternion(q: Quaternion): this;
    project(camera: PerspectiveCamera): this;
    setFromUnitVectors(from: Vector3, to: Vector3): this;
  }

  export class Quaternion {
    identity(): this;
    slerp(q: Quaternion, t: number): this;
    setFromUnitVectors(from: Vector3, to: Vector3): this;
  }

  export class Color {
    set(css: string): this;
    getRGB(
      target: { r: number; g: number; b: number },
      colorSpace?: string,
    ): { r: number; g: number; b: number };
  }

  export class Object3D {
    scale: { set(x: number, y: number, z: number): void };
    position: Vector3;
    frustumCulled: boolean;
    renderOrder: number;
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

  export class Scene {
    add(object: Object3D): void;
    remove(object: Object3D): void;
  }

  export class BufferAttribute {
    needsUpdate: boolean;
    constructor(array: Float32Array | Uint32Array, itemSize: number);
    setUsage(usage: number): this;
  }

  export class BufferGeometry {
    setAttribute(name: string, attribute: BufferAttribute): this;
    setIndex(attribute: BufferAttribute): this;
    dispose(): void;
  }

  export class ShaderMaterial {
    uniforms: Record<string, { value: number }>;
    constructor(params?: Record<string, unknown>);
    dispose(): void;
  }

  export class Points extends Object3D {
    geometry: BufferGeometry;
    constructor(geometry: BufferGeometry, material: ShaderMaterial);
  }

  export class LineSegments extends Object3D {
    geometry: BufferGeometry;
    constructor(geometry: BufferGeometry, material: ShaderMaterial);
  }

  export class PerspectiveCamera extends Object3D {
    aspect: number;
    fov: number;
    constructor(fov: number, aspect: number, near: number, far: number);
    updateProjectionMatrix(): void;
    updateMatrixWorld(): void;
    getWorldDirection(target: Vector3): Vector3;
  }

  export class WebGLRenderer {
    domElement: HTMLCanvasElement;
    constructor(params?: Record<string, unknown>);
    setPixelRatio(value: number): void;
    setSize(width: number, height: number, updateStyle?: boolean): void;
    setClearColor(color: number, alpha?: number): void;
    render(scene: Scene, camera: PerspectiveCamera): void;
    dispose(): void;
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

declare module "three/examples/jsm/controls/OrbitControls.js" {
  import type { PerspectiveCamera, Vector3 } from "three";

  export class OrbitControls {
    constructor(camera: PerspectiveCamera, domElement: HTMLElement);
    enableDamping: boolean;
    dampingFactor: number;
    rotateSpeed: number;
    zoomSpeed: number;
    minDistance: number;
    maxDistance: number;
    zoomToCursor: boolean;
    enableRotate: boolean;
    screenSpacePanning: boolean;
    autoRotate: boolean;
    autoRotateSpeed: number;
    mouseButtons: { LEFT: number; MIDDLE: number; RIGHT: number };
    touches: { ONE: number; TWO: number };
    target: Vector3;
    update(): void;
    dispose(): void;
  }
}
