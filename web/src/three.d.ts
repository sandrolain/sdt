/**
 * Minimal ambient types for the three.js surface used by the viewer.
 *
 * The project deliberately does not depend on `@types/three`; this stub declares
 * only the members the 3D graph and the shared JsonCanvas 3D layer touch.
 * Extend it when new three.js members are adopted.
 */
declare module "three" {
  export const SRGBColorSpace: string;
  export const LinearFilter: number;
  export const DynamicDrawUsage: number;
  export const DoubleSide: number;
  export const FrontSide: number;

  export const MOUSE: { PAN: number; DOLLY: number; ROTATE: number };
  export const TOUCH: { PAN: number; DOLLY_PAN: number; ROTATE: number };

  export const MathUtils: {
    degToRad(value: number): number;
  };

  export class Vector2 {
    x: number;
    y: number;
    constructor(x?: number, y?: number);
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
    r: number;
    g: number;
    b: number;
    constructor(value?: string | number);
    set(css: string): this;
    getRGB(
      target: { r: number; g: number; b: number },
      colorSpace?: string,
    ): { r: number; g: number; b: number };
  }

  export class Object3D {
    scale: { set(x: number, y: number, z: number): void };
    position: Vector3;
    quaternion: Quaternion;
    userData: Record<string, unknown>;
    visible: boolean;
    frustumCulled: boolean;
    renderOrder: number;
    add(child: Object3D): this;
    remove(child: Object3D): this;
    traverse(callback: (object: Object3D) => void): void;
  }

  export class Sprite extends Object3D {
    constructor(material?: SpriteMaterial);
  }

  export class SpriteMaterial {
    constructor(params?: Record<string, unknown>);
  }

  export class Material {
    dispose(): void;
  }

  export class Texture extends Material {
    dispose(): void;
  }

  export class CanvasTexture extends Texture {
    colorSpace: string;
    minFilter: number;
    anisotropy: number;
    generateMipmaps: boolean;
    constructor(canvas: HTMLCanvasElement);
  }

  export class Scene {
    add(object: Object3D): void;
    remove(object: Object3D): void;
    traverse(callback: (object: Object3D) => void): void;
  }

  export class BufferAttribute {
    needsUpdate: boolean;
    constructor(array: Float32Array | Uint32Array, itemSize: number);
    setUsage(usage: number): this;
  }

  export class BufferGeometry {
    setAttribute(name: string, attribute: BufferAttribute): this;
    setIndex(attribute: BufferAttribute): this;
    setFromPoints(points: Vector3[]): this;
    dispose(): void;
  }

  export class PlaneGeometry extends BufferGeometry {
    constructor(width?: number, height?: number);
  }

  export class EdgesGeometry extends BufferGeometry {
    constructor(geometry: BufferGeometry);
  }

  export class ConeGeometry extends BufferGeometry {
    constructor(radius?: number, height?: number, radialSegments?: number);
  }

  export class ShaderMaterial extends Material {
    uniforms: Record<string, { value: number }>;
    constructor(params?: Record<string, unknown>);
  }

  export class MeshBasicMaterial extends Material {
    color: Color | string;
    map: CanvasTexture | null;
    transparent: boolean;
    side: number;
    depthWrite: boolean;
    opacity: number;
    constructor(params?: Record<string, unknown>);
  }

  export class LineBasicMaterial extends Material {
    color: Color | string;
    transparent: boolean;
    opacity: number;
    constructor(params?: Record<string, unknown>);
  }

  export class Points extends Object3D {
    geometry: BufferGeometry;
    constructor(geometry: BufferGeometry, material: ShaderMaterial);
  }

  export class LineSegments extends Object3D {
    geometry: BufferGeometry;
    constructor(geometry: BufferGeometry, material: Material);
  }

  export class Line extends Object3D {
    geometry: BufferGeometry;
    material: Material;
    constructor(geometry: BufferGeometry, material: Material);
  }

  export class Mesh extends Object3D {
    geometry: BufferGeometry;
    material: MeshBasicMaterial;
    constructor(geometry?: BufferGeometry, material?: MeshBasicMaterial);
  }

  export class CubicBezierCurve3 {
    constructor(v0: Vector3, v1: Vector3, v2: Vector3, v3: Vector3);
    getPoints(divisions?: number): Vector3[];
    getPoint(t: number, optionalTarget?: Vector3): Vector3;
    getTangent(t: number, optionalTarget?: Vector3): Vector3;
  }

  export class Raycaster {
    setFromCamera(coords: Vector2, camera: PerspectiveCamera): void;
    intersectObjects(objects: Object3D[], recursive?: boolean): { object: Object3D }[];
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
