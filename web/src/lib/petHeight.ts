/**
 * The pet band's height, as the reader left it.
 *
 * The same shape as the panel's width (`lib/agentPanelWidth.ts`): a browser preference in
 * `localStorage`, written at the end of a gesture rather than per pointer move, never a
 * project file. The stored number is in CSS pixels, which is what the drag measures; the
 * stylesheet keeps the same floor, default and ceiling for the band it paints.
 */

export const PET_HEIGHT_KEY = "sdt-agent-pet-height";

/** 5rem, 11rem and 24rem at the app's 15px root — the numbers `--agent-pet-h` is held between. */
export const MIN_PET_HEIGHT = 75;
export const DEFAULT_PET_HEIGHT = 165;
export const MAX_PET_HEIGHT = 360;

/** clampPetHeight holds a height between the band's floor and its ceiling. */
export function clampPetHeight(height: number): number {
  if (!Number.isFinite(height)) return DEFAULT_PET_HEIGHT;
  return Math.min(MAX_PET_HEIGHT, Math.max(MIN_PET_HEIGHT, Math.round(height)));
}

/** readPetHeight reads the remembered height; anything unusable is the default, not an error. */
export function readPetHeight(): number {
  const raw = typeof localStorage !== "undefined" ? localStorage.getItem(PET_HEIGHT_KEY) : null;
  if (raw === null) return DEFAULT_PET_HEIGHT;
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) ? clampPetHeight(parsed) : DEFAULT_PET_HEIGHT;
}

/** writePetHeight remembers the height, clamped so a corrupt value can never be stored. */
export function writePetHeight(height: number): void {
  localStorage.setItem(PET_HEIGHT_KEY, String(clampPetHeight(height)));
}
