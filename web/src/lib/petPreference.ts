/**
 * The pet, as the reader left it.
 *
 * Two browser preferences beside the panel's width (`lib/agentPanelWidth.ts`): whether the
 * pet shows at all, and which variant it is. A preference, never a project file — the
 * viewer does not write `.sdt.yaml` — so the pet is the reader's and not the repository's.
 *
 * On the light theme the two additive WebGL variants fall back to `dot`: `sciame` and
 * `trama` fade on the Latte mantle. The 2D characters are designed for both themes, so a 2D
 * choice is shown as chosen.
 */
import type { PetVariant } from "./pet/types";

export const PET_ENABLED_KEY = "sdt-agent-pet";
export const PET_VARIANT_KEY = "sdt-agent-pet-variant";

/** The six variants, in the order the menu offers them: WebGL first, then Canvas 2D. */
export const PET_VARIANTS: readonly PetVariant[] = [
  "dot",
  "sciame",
  "trama",
  "fiamma",
  "scriptor",
  "possibile",
];

/** The pet is on unless the reader turned it off: the panel's face is the default. */
export function readPetEnabled(): boolean {
  if (typeof localStorage === "undefined") return true;
  return localStorage.getItem(PET_ENABLED_KEY) !== "off";
}

/** writePetEnabled remembers the reader's choice; `true` removes the key (the default). */
export function writePetEnabled(enabled: boolean): void {
  if (enabled) localStorage.removeItem(PET_ENABLED_KEY);
  else localStorage.setItem(PET_ENABLED_KEY, "off");
}

/** readPetVariant reads the remembered variant; anything unrecognised is `dot`. */
export function readPetVariant(): PetVariant {
  if (typeof localStorage === "undefined") return "dot";
  const raw = localStorage.getItem(PET_VARIANT_KEY);
  return PET_VARIANTS.find((variant) => variant === raw) ?? "dot";
}

/** writePetVariant remembers the chosen variant, refusing a value the pet does not have. */
export function writePetVariant(variant: PetVariant): void {
  if (!PET_VARIANTS.includes(variant)) return;
  localStorage.setItem(PET_VARIANT_KEY, variant);
}

/**
 * effectiveVariant is the variant that can actually be painted: the two additive WebGL
 * variants need the dark mantle, so the light theme gets the dot whatever was chosen. A 2D
 * character is designed for both themes and is never replaced.
 */
export function effectiveVariant(chosen: PetVariant, theme: "light" | "dark"): PetVariant {
  if (theme === "light" && (chosen === "sciame" || chosen === "trama")) return "dot";
  return chosen;
}
