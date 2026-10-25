/**
 * The conversation's state as a word.
 *
 * The pet shows the state as motion, but motion is not readable to a screen reader and the
 * pet is a browser preference: this is the same derivation said in words, rendered in the
 * band's corner while the pet is on and in the panel's header while it is off. It never
 * derives a state of its own — `usePetState` owns the machine, so the word cannot disagree
 * with the animation.
 */
import type { PetState } from "../lib/pet/types";
import { stateWord, type PetPhase } from "../lib/petState";

export function AgentStateWord({
  phase,
  state,
  className,
}: {
  phase: PetPhase;
  state: PetState;
  className?: string;
}) {
  return (
    <p className={`agent-panel__state-word${className ? ` ${className}` : ""}`}>
      {stateWord(phase, state)}
    </p>
  );
}
