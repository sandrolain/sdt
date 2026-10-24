/**
 * The pet, as the reader left it.
 *
 * Two browser preferences beside the panel's width (`lib/agentPanelWidth.ts`): whether the
 * pet shows at all, and which variant it is. A preference, never a project file — the
 * viewer does not write `.sdt.yaml` — so the pet is the reader's and not the repository's.
 *
 * On the light theme the answer is always `dot`: `sciame` and `trama` are additive and fade
 * on the Latte mantle, so the choice is kept but not shown until the theme can carry it.
 */
import type { PetVariant } from "./pet/types";

export const PET_ENABLED_KEY = "sdt-agent-pet";
export const PET_VARIANT_KEY = "sdt-agent-pet-variant";

/** The three variants, in the order the menu offers them. */
export const PET_VARIANTS: readonly PetVariant[] = ["dot", "sciame", "trama"];

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
 * effectiveVariant is the variant that can actually be painted: the additive ones need the
 * dark mantle, so the light theme gets the dot whatever was chosen.
 */
export function effectiveVariant(chosen: PetVariant, theme: "light" | "dark"): PetVariant {
  return theme === "light" ? "dot" : chosen;
}
