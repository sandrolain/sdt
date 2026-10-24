/**
 * The pet's band: a fixed-height strip above the composer that gives the agent panel a face.
 *
 * It is lazy-loaded (the panel `lazy()`s this module) so `three` never joins the entry
 * chunk, and it holds no conversation state of its own: `web/src/lib/petState.ts` derives the
 * pet's state from the phase and the thread, this component applies it and owns the clock.
 *
 * Motion follows the doctrine: the loop rests when the agent rests (`idle`/`sleeping`), a
 * reduced-motion reader gets one settled frame per state change, and a hidden document
 * pauses the canvas. The pet is decorative — the state is already words in the thread — so
 * the band is `aria-hidden` and the header's toggle carries the only accessible name.
 */
import { useEffect, useMemo, useRef, useState } from "react";
import type { AgentThread } from "../lib/agent";
import { PetCanvas, type PetHandle } from "../lib/pet";
import type { PetOptions, PetState, PetVariant } from "../lib/pet/types";
import {
  ERROR_MS,
  MIN_STATE_MS,
  SLEEP_AFTER_MS,
  SUCCESS_MS,
  nextPetState,
  petPulse,
  petSignal,
  type PetFrame,
  type PetPhase,
  type PetSignal,
} from "../lib/petState";

/** The library's own guidance for a small stage, measured against the 11rem band. */
const PET_OPTIONS: PetOptions = { density: 0.6, pixelRatioCap: 1.5 };

/**
 * The clock, read only from the effect below: the state machine takes time as a parameter,
 * so nothing reads "now" during render (the app's date-purity rule). A module-scope arrow
 * is the documented escape hatch for a non-render read.
 */
const clock = () => performance.now();

export function PetStrip({
  phase,
  thread,
  variant,
}: {
  phase: PetPhase;
  thread?: AgentThread;
  variant: PetVariant;
}) {
  const pet = useRef<PetHandle>(null);
  const frame = useRef<PetFrame | null>(null);
  const previous = useRef<PetSignal | null>(null);
  const [state, setState] = useState<PetState>("idle");
  const [unavailable, setUnavailable] = useState(false);
  const reducedMotion = usePrefersReducedMotion();
  const hidden = useDocumentHidden();

  const running = Boolean(thread?.running);
  const signal = useMemo(() => petSignal(phase, thread, running), [phase, thread, running]);

  // The state machine runs once per signal and schedules the one moment it could move
  // again: a held change, the end of a celebration, or the rest before sleep. Nothing here
  // polls — a state with no pending change sets no timer.
  useEffect(() => {
    let timer = 0;
    const step = () => {
      const now = clock();
      const before = frame.current;
      const next = nextPetState(before, signal, now);
      if (next !== before) {
        frame.current = next;
        setState(next.state);
      }
      window.clearTimeout(timer);
      const due = dueAt(next, next === before);
      if (due !== null) timer = window.setTimeout(step, Math.max(0, due - now));
    };
    step();
    return () => window.clearTimeout(timer);
  }, [signal]);

  // A pulse is an answer growing during a turn: the only thing the pet needs from the
  // previous signal, so it is kept beside the machine rather than inside it.
  useEffect(() => {
    const amount = petPulse(previous.current, signal);
    previous.current = signal;
    if (amount > 0) pet.current?.pulse(amount);
  }, [signal]);

  if (unavailable) return null;

  // A resting pet and a reduced-motion reader both get still frames; a hidden document
  // pauses without settling, so the pet comes back exactly as it was.
  const still = reducedMotion || state === "idle" || state === "sleeping";

  return (
    <div className="agent-panel__pet" aria-hidden="true">
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
  );
}

/**
 * dueAt is the next moment the frame could change on its own, or null when the next change
 * needs a new signal. `held` is a frame the machine refused to leave yet: the flicker guard
 * is what expires.
 */
function dueAt(frame: PetFrame, held: boolean): number | null {
  if (held) return frame.at + MIN_STATE_MS;
  if (frame.state === "success") return frame.at + SUCCESS_MS;
  if (frame.state === "error") return frame.at + ERROR_MS;
  if (frame.state === "idle") return frame.idleSince + SLEEP_AFTER_MS;
  return null;
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
