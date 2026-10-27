/**
 * Creates the pet of one Canvas 2D variant on a canvas.
 *
 * The WebGL factory (`../pet/createPet`) is the other half of the same vocabulary; this one
 * exists so a 2D choice can be resolved without importing `three`.
 */
import type { Pet2DVariant, PetController, PetOptions, PetTheme } from "../pet/types";
import { FiammaPet } from "./variants/fiamma";
import { PossibilePet } from "./variants/possibile";
import { ScriptorPet } from "./variants/scriptor";

export function createPet2D(
  variant: Pet2DVariant,
  canvas: HTMLCanvasElement,
  options: PetOptions = {},
  theme: PetTheme = "dark",
): PetController {
  switch (variant) {
    case "fiamma":
      return new FiammaPet(canvas, options, theme);
    case "scriptor":
      return new ScriptorPet(canvas, options, theme);
    case "possibile":
      return new PossibilePet(canvas, options, theme);
  }
}
