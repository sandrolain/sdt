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
export type PetVariant = "dot" | "sciame" | "trama";

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
  dispose(): void;
}
