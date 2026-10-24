/**
 * Creates the pet of one variant on a canvas.
 *
 * Vendored from `context/refs/react/pet-react/createPet.ts`; the switch is the variant
 * vocabulary and is unchanged.
 */
import type { PetController, PetOptions, PetVariant } from "./types";
import { DotPet } from "./variants/dot";
import { SciamePet } from "./variants/sciame";
import { TramaPet } from "./variants/trama";

export function createPet(
  variant: PetVariant,
  canvas: HTMLCanvasElement,
  options: PetOptions = {},
): PetController {
  switch (variant) {
    case "dot":
      return new DotPet(canvas, options);
    case "sciame":
      return new SciamePet(canvas, options);
    case "trama":
      return new TramaPet(canvas, options);
  }
}
