/**
 * The Canvas 2D pet family: Fiamma, Scriptor and Possibile.
 *
 * Nothing here imports `three`; the factory is reached through a dynamic import from
 * `PetCanvas`, so a reader who picks a 2D pet never downloads the WebGL library.
 */
export { createPet2D } from "./createPet";
export { Pet2D } from "./Pet2D";
export { FiammaPet } from "./variants/fiamma";
export { ScriptorPet } from "./variants/scriptor";
export { PossibilePet } from "./variants/possibile";
