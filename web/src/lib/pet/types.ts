/**
 * The pet's vocabulary: the eight states, the three variants and the controller.
 *
 * Vendored from `context/refs/react/pet-react/types.ts`; the code is this project's own
 * material, not a third-party licence. Comments are translated, names and behaviour are
 * unchanged.
 */

export const PET_STATES = [
  "idle",
  "listening",
  "thinking",
  "working",
  "speaking",
  "success",
  "error",
  "sleeping",
] as const;

export type PetState = (typeof PET_STATES)[number];
export type PetVariant = "dot" | "sciame" | "trama" | "fiamma" | "scriptor" | "possibile";
/** The three WebGL variants: the ones a `three` chunk can paint. */
export type PetWebGLVariant = "dot" | "sciame" | "trama";
/** The three Canvas 2D variants: the ones that must never load `three`. */
export type Pet2DVariant = "fiamma" | "scriptor" | "possibile";

export const PET_2D_VARIANTS: readonly Pet2DVariant[] = ["fiamma", "scriptor", "possibile"];

export function is2DVariant(variant: PetVariant): variant is Pet2DVariant {
  return (PET_2D_VARIANTS as readonly string[]).includes(variant);
}
/** The panel's theme, as a controller that repaints needs it. */
export type PetTheme = "light" | "dark";

export interface PetOptions {
  /** Ceiling for the canvas pixel ratio (default 2). */
  pixelRatioCap?: number;
  /** Multiplier for the particle/node density (default 1). Use ~0.5 on weak devices. */
  density?: number;
  /** Renderer antialiasing (default true). */
  antialias?: boolean;
}

/** Public API, identical for every variant. */
export interface PetController {
  readonly state: PetState;
  /** Changes state. One-shot states (success, error) return to idle by themselves. */
  setState(state: PetState): void;
  /** An energy pulse, e.g. on every streamed token. */
  pulse(amount?: number): void;
  /** Gaze direction, normalised from -1 to 1. */
  lookAt(x: number, y: number): void;
  /** Pauses or resumes the render loop (e.g. while off screen). */
  setPaused(paused: boolean): void;
  /**
   * Switches between the animated loop and still frames. When still, the loop stops, the
   * interpolation snaps to the target preset and each state change paints exactly one
   * frame — what reduced motion and a resting pet need, and what `setPaused` cannot do.
   */
  setStill(still: boolean): void;
  resize(width: number, height: number): void;
  on(event: "stateChange", cb: (state: PetState) => void): () => void;
  /**
   * Repaints for a theme, keeping the state and the clock. Optional: the WebGL variants mix
   * their colour once, at construction, and a 2D character derives its ink per frame.
   */
  setTheme?(theme: PetTheme): void;
  dispose(): void;
}
