/**
 * The pet library, vendored for the agent panel.
 *
 * Source: `context/refs/react/pet-react/` (this project's own material; the prototypes in
 * `context/refs/react/pet/` are its origin). The copy is behaviour-identical to the
 * reference — the three `STATES` tables are untouched — with comments translated and the
 * still mode the panel's motion doctrine needs.
 *
 * `agentAdapter.ts` is deliberately **not** vendored: it translates a stream of events
 * (`request`, `tool_call`, `token`, `done`), while the panel holds a whole-thread snapshot.
 * `web/src/lib/petState.ts` is the mapper that shape deserves.
 *
 * `three` is imported statically by these modules, so this folder must only ever be reached
 * through a lazy import (the panel's strip), never from the entry chunk.
 */
export { PetCanvas } from "./PetCanvas";
export type { PetCanvasProps, PetHandle } from "./PetCanvas";
export { createPet } from "./createPet";
export { PET_STATES } from "./types";
export type { PetController, PetOptions, PetState, PetVariant } from "./types";
