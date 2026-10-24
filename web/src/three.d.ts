/**
 * Minimal ambient types for the three.js surface used by the viewer.
 *
 * The project deliberately does not depend on `@types/three`; this stub declares
 * only the members the 3D graph, the shared JsonCanvas 3D layer and the agent panel's
 * pet touch. Extend it when new three.js members are adopted.
 *
 * The pet's variants are shader work: they bind `Color` and `Vector2` uniforms, use a
 * `ShaderMaterial` on meshes and points, and rotate groups. Those are the widenings in
 * this file — each additive, so the graph engine's existing uses stay assignable.
 */
declare module "three" {
  export const SRGBColorSpace: string;
  export const LinearSRGBColorSpace: string;
  export const LinearFilter: number;
  export const DynamicDrawUsage: number;
  export const AdditiveBlending: number;
  export const DoubleSide: number;
  export const FrontSide: number;

  export const MOUSE: { PAN: number; DOLLY: number; ROTATE: number };
  export const TOUCH: { PAN: number; DOLLY_PAN: number; ROTATE: number };

  export const MathUtils: {
    degToRad(value: number): number;
  };

  /** One shader uniform. The value is the shader's own type: a number, a Color, a Vector2. */
  export interface IUniform<T = any> {
    value: T;
  }

  export class Vector2 {
    x: number;
    y: number;
    constructor(x?: number, y?: number);
    set(x: number, y: number): this;
    multiplyScalar(s: number): this;
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
    dot(v: Vector3): number;
    length(): number;
    distanceTo(v: Vector3): number;
    distanceToSquared(v: Vector3): number;
    divideScalar(s: number): this;
    multiplyScalar(s: number): this;
    normalize(): this;
    addScaledVector(v: Vector3, s: number): this;
    applyQuaternion(q: Quaternion): this;
    project(camera: PerspectiveCamera): this;
    setFromUnitVectors(from: Vector3, to: Vector3): this;
  }

  export class Quaternion {
    identity(): this;
    slerp(q: Quaternion, t: number): this;
    setFromUnitVectors(from: Vector3, to: Vector3): this;
  }

  /** A rotation in the three axes: what a group or a mesh is turned with. */
  export class Euler {
    x: number;
    y: number;
    z: number;
    set(x: number, y: number, z: number): this;
  }

  export class Color {
    r: number;
    g: number;
    b: number;
    constructor(value?: string | number);
    set(css: string): this;
    setStyle(css: string, colorSpace?: string): this;
    setHex(hex: number, colorSpace?: string): this;
    copy(color: Color): this;
    lerp(color: Color, alpha: number): this;
    getRGB(
      target: { r: number; g: number; b: number },
      colorSpace?: string,
    ): { r: number; g: number; b: number };
  }

  export class Object3D {
    scale: Vector3;
    position: Vector3;
    rotation: Euler;
    quaternion: Quaternion;
    userData: Record<string, unknown>;
    visible: boolean;
    frustumCulled: boolean;
    renderOrder: number;
    add(...children: Object3D[]): this;
    remove(child: Object3D): this;
    traverse(callback: (object: Object3D) => void): void;
  }

  /** A container with no geometry: the pet's variants hang the body and the eyes off one. */
  export class Group extends Object3D {}

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
    add(...objects: Object3D[]): void;
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

  export class IcosahedronGeometry extends BufferGeometry {
    constructor(radius?: number, detail?: number);
  }

  export class SphereGeometry extends BufferGeometry {
    constructor(radius?: number, widthSegments?: number, heightSegments?: number);
  }

  export class ShaderMaterial extends Material {
    uniforms: Record<string, IUniform>;
    constructor(params?: Record<string, unknown>);
  }

  export class MeshBasicMaterial extends Material {
    color: Color;
    map: CanvasTexture | null;
    transparent: boolean;
    side: number;
    depthWrite: boolean;
    opacity: number;
    constructor(params?: Record<string, unknown>);
  }

  export class LineBasicMaterial extends Material {
    color: Color;
    transparent: boolean;
    opacity: number;
    constructor(params?: Record<string, unknown>);
  }

  export class Points extends Object3D {
    geometry: BufferGeometry;
    constructor(geometry: BufferGeometry, material: Material);
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
    material: Material | Material[];
    constructor(geometry?: BufferGeometry, material?: Material | Material[]);
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
    outputColorSpace: string;
    constructor(params?: Record<string, unknown>);
    getPixelRatio(): number;
    setPixelRatio(value: number): void;
    setSize(width: number, height: number, updateStyle?: boolean): void;
    setClearColor(color: number, alpha?: number): void;
    forceContextLoss(): void;
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
