/**
 * How long each pet's endings last.
 *
 * The library's defaults (1800 ms for success, 1300 ms for error) are a WebGL constant, and
 * they truncated Scriptor's word in its prototype: a typewriter needs 2600 ms to finish.
 * The ending belongs to the character, so the table lives beside the vocabulary and the
 * panel hands the chosen variant's pair to the state machine.
 */
import type { PetVariant } from "./pet/types";
import type { PetTiming } from "./petState";

export interface PetOnce {
  successMs: number;
  errorMs: number;
}

export const PET_ONCE: Record<PetVariant, PetOnce> = {
  dot: { successMs: 1_800, errorMs: 1_300 },
  sciame: { successMs: 1_800, errorMs: 1_300 },
  trama: { successMs: 1_800, errorMs: 1_300 },
  fiamma: { successMs: 2_400, errorMs: 1_800 },
  scriptor: { successMs: 2_600, errorMs: 2_000 },
  possibile: { successMs: 2_400, errorMs: 1_800 },
};

/** The machine's timing for one variant. */
export function petTimingFor(variant: PetVariant): PetTiming {
  return PET_ONCE[variant];
}
