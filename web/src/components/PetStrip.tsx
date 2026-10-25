/**
 * The pet's band: a fixed-height strip above the composer that gives the agent panel a face.
 *
 * It is lazy-loaded (the panel `lazy()`s this module) so `three` never joins the entry
 * chunk. The state machine itself lives in `web/src/lib/petState.ts` (`usePetState`) and the
 * panel owns it, because the state is also said in words — this component only applies the
 * state it is handed and owns the pulse.
 *
 * Motion follows the doctrine: the loop rests when the agent rests (`idle`/`sleeping`), a
 * reduced-motion reader gets one settled frame per state change, and a hidden document
 * pauses the canvas. The canvas is decorative and hidden from assistive tech; the band's
 * corner carries the same state as a word (`AgentStateWord`).
 */
import { useEffect, useRef, useState } from "react";
import { PetCanvas, type PetHandle } from "../lib/pet";
import type { PetOptions, PetState, PetVariant } from "../lib/pet/types";
import { petPulse, type PetPhase, type PetSignal } from "../lib/petState";
import { AgentStateWord } from "./AgentStateWord";

/** The library's own guidance for a small stage, measured against the 11rem band. */
const PET_OPTIONS: PetOptions = { density: 0.6, pixelRatioCap: 1.5 };

export function PetStrip({
  phase,
  signal,
  state,
  variant,
}: {
  phase: PetPhase;
  signal: PetSignal;
  state: PetState;
  variant: PetVariant;
}) {
  const pet = useRef<PetHandle>(null);
  const previous = useRef<PetSignal | null>(null);
  const [unavailable, setUnavailable] = useState(false);
  const reducedMotion = usePrefersReducedMotion();
  const hidden = useDocumentHidden();

  // A pulse is an answer growing during a turn: the only thing the pet needs from the
  // previous signal, so it is kept beside the machine rather than inside it.
  useEffect(() => {
    const amount = petPulse(previous.current, signal);
    previous.current = signal;
    if (amount > 0) pet.current?.pulse(amount);
  }, [signal]);

  // A resting pet and a reduced-motion reader both get still frames; a hidden document
  // pauses without settling, so the pet comes back exactly as it was.
  const still = reducedMotion || state === "idle" || state === "sleeping";

  return (
    <div className="agent-panel__pet">
      {unavailable ? null : (
        <div className="agent-panel__pet-stage" aria-hidden="true">
          <PetCanvas
            ref={pet}
            variant={variant}
            state={state}
            options={PET_OPTIONS}
            still={still}
            paused={hidden}
            onUnavailable={() => setUnavailable(true)}
          />
        </div>
      )}
      <AgentStateWord phase={phase} state={state} className="agent-panel__state-word--band" />
    </div>
  );
}

/** The reduced-motion preference, following the system setting while the panel is open. */
function usePrefersReducedMotion(): boolean {
  const query = () =>
    typeof matchMedia === "function" ? matchMedia("(prefers-reduced-motion: reduce)") : null;
  const [reduced, setReduced] = useState(() => query()?.matches ?? false);
  useEffect(() => {
    const mq = query();
    if (!mq) return;
    const onChange = () => setReduced(mq.matches);
    mq.addEventListener?.("change", onChange);
    return () => mq.removeEventListener?.("change", onChange);
  }, []);
  return reduced;
}

/** Whether the document is hidden: a background tab stops the loop. */
function useDocumentHidden(): boolean {
  const [hidden, setHidden] = useState(() => typeof document !== "undefined" && document.hidden);
  useEffect(() => {
    const onChange = () => setHidden(document.hidden);
    document.addEventListener("visibilitychange", onChange);
    return () => document.removeEventListener("visibilitychange", onChange);
  }, []);
  return hidden;
}

export default PetStrip;
