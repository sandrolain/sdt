/**
 * AgentPanel — the agent bridge as a slide-in panel over any screen.
 *
 * The panel is the app's own overlay idiom (`Modal` + `Dialog`, the pair `SearchPalette`
 * and `GraphGuide` already use) rather than a route, so a reader can watch the map or read
 * a document and talk to the agent beside it. It is anchored to the inline end, the edge
 * its trigger sits on, and arrives with the app's one motion.
 *
 * It asks the reader for nothing. The agent is the one the project declared in `.sdt.yaml`
 * and the viewer starts it; this component reports what the *process* is doing — starting,
 * working, starting again after a death, or stopped with a reason and a way to ask for
 * another start — because the bridge is the only thing that knows, and a panel that
 * guessed would disagree with it exactly when a restart is in flight.
 *
 * Its width is the reader's too: the handle on the panel's inline-start edge drags and
 * steps it, and the value is remembered across sessions. The pet band under the header
 * carries the same idea on the other axis: its lower edge is a handle, and its height is a
 * browser preference.
 */
import {
  Button,
  Dialog,
  Menu,
  MenuItem,
  MenuTrigger,
  Modal,
  ModalOverlay,
  Popover,
} from "react-aria-components";
import { ComposerPrimitive, useAui } from "@assistant-ui/react";
import { AgentThread } from "./AgentThread";
import { AgentStateWord } from "./AgentStateWord";
import { ComboBox, type UiComboOption } from "./ui/ComboBox";
import { TooltipButton } from "./ui/Tooltip";
import { Icon } from "../lib/icon";
import {
  effectiveVariant,
  PET_VARIANTS,
  readPetEnabled,
  readPetVariant,
  writePetEnabled,
  writePetVariant,
} from "../lib/petPreference";
import type { PetVariant } from "../lib/pet/types";
import { petSignal, usePetState } from "../lib/petState";
import { petTimingFor } from "../lib/petTimings";
import { THEME_ICON, THEME_LABEL } from "../lib/theme";
import { useDocumentTheme } from "../lib/themeAttr";
import { useThemeOptional } from "../lib/useTheme";
import { is2DVariant } from "../lib/pet/types";
import {
  Suspense,
  lazy,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
} from "react";
import {
  clampPanelWidth,
  DEFAULT_PANEL_WIDTH,
  MAX_PANEL_WIDTH,
  MIN_PANEL_WIDTH,
  readPanelWidth,
  writePanelWidth,
} from "../lib/agentPanelWidth";
import {
  clampPetHeight,
  DEFAULT_PET_HEIGHT,
  MAX_PET_HEIGHT,
  MIN_PET_HEIGHT,
  readPetHeight,
  writePetHeight,
} from "../lib/petHeight";
import {
  activateAgent,
  cancelCommand,
  configCommand,
  fetchAgentState,
  modelOption,
  newAgentChat,
  offeredModels,
  elicitationCommand,
  PART_TEXT,
  permissionCommand,
  promptCommand,
  ROLE_USER,
  sendAgentCommand,
  streamAgent,
  type AgentState,
  type AgentStateEnvelope,
  type AgentStatusFrame,
  type AgentStream,
} from "../lib/agent";
import { caretAtEdge, NO_WALK, recall, type ComposerWalk } from "../lib/composerHistory";

/**
 * The pet is lazy for the same reason the canvas is: its library imports `three`, and a
 * reader who never opens the panel never downloads it. The band declares its own height, so
 * the chunk arriving never resizes the thread.
 */
const PetStrip = lazy(() => import("./PetStrip"));

/** What the panel is showing, as one value rather than a set of booleans. */
type Phase = "loading" | "starting" | "active" | "restarting" | "stopped" | "error";

/**
 * AgentCommand is one command the agent advertises, as the bridge folds it
 * (`thread.commands`): the name the reader types and the sentence that explains it.
 */
type AgentCommand = { name: string; description?: string };

/**
 * phaseFrom reads the phase from the viewer's account of the process. A failure and a stop
 * are different things to a reader: a failure carries a sentence and offers a retry, a
 * stop is simply off.
 */
function phaseFrom(state: AgentState): Phase {
  switch (state.status) {
    case "starting":
      return "starting";
    case "active":
      return state.connected ? "active" : "starting";
    case "restarting":
      return "restarting";
    case "idle":
    case "stopped":
      return state.error && !state.activated ? "error" : "stopped";
    default:
      return state.activated ? "active" : "stopped";
  }
}

export function AgentPanel({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [phase, setPhase] = useState<Phase>("loading");
  const [state, setState] = useState<AgentState | null>(null);
  const [message, setMessage] = useState("");
  const [envelope, setEnvelope] = useState<AgentStateEnvelope | null>(null);
  const [session, setSession] = useState("");
  const stream = useRef<AgentStream | null>(null);

  /** apply takes the viewer's own account of the process as the truth about it. */
  const apply = (next: AgentState) => {
    setState(next);
    setMessage(next.error ?? "");
    setPhase(phaseFrom(next));
  };

  /**
   * applyStatus folds a posture frame into the same account a state read fills: one
   * function decides the phase, so the stream and the endpoint cannot disagree. The
   * frame carries no `command` (it does not change while a stream is open), so the
   * panel keeps the one it already has.
   */
  const applyStatus = (frame: AgentStatusFrame) => {
    apply({
      activated: frame.activated,
      connected: frame.connected,
      status: frame.status,
      session: frame.sessionId,
      model: frame.model,
      error: frame.error,
      command: state?.command,
      // A posture frame carries no declaration: it is config, and it does not change while
      // a stream is open.
      models: state?.models,
    });
  };

  const fail = (reason: string) => {
    setMessage(reason);
    setPhase("error");
  };

  /** readState re-reads the bridge's posture; it runs from a click, so it may set loading. */
  const readState = async () => {
    setPhase("loading");
    try {
      apply(await fetchAgentState());
    } catch (error) {
      fail((error as Error).message);
    }
  };

  // Opening the panel is what starts the flow: nothing is asked of the viewer, and
  // nothing is started, before it. A stale answer from a previous open is dropped rather
  // than applied.
  useEffect(() => {
    if (!open) return;
    let live = true;
    fetchAgentState()
      .then((next) => {
        if (live) apply(next);
      })
      .catch((error: Error) => {
        if (live) fail(error.message);
      });
    return () => {
      live = false;
    };
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps -- apply/fail only set state

  // One stream for the page, opened when the agent is live and closed when it is not: a
  // conversation lives as long as the viewer does, not as long as the panel is on screen,
  // so reopening the panel shows the thread as it stands.
  useEffect(() => {
    if (phase !== "active") return;
    const opened = streamAgent({
      onState: (next) => {
        setEnvelope(next);
        setSession(next.sessionId);
      },
      onStatus: applyStatus,
      onClose: () => {
        // The stream ends when the agent it was carrying is gone — a restart, a stop — and
        // the bridge, not this panel, knows which one it was: re-reading its posture is what
        // turns an interruption into "starting it again…" instead of into a dead panel.
        void readState();
      },
    });
    stream.current = opened;
    return () => {
      opened.close();
      stream.current = null;
    };
  }, [phase]);

  // The posture arrives on the stream, so the panel has no polling of its own: a
  // restart that begins and ends between two changes is still a change, and the
  // bridge reports it. The viewer's liveness tick stays where it is — it ends a
  // stream whose agent is gone, which is a different job from reporting a change.

  /** start asks the viewer to start the agent the project declared. */
  const start = async () => {
    setPhase("starting");
    setMessage("");
    try {
      apply(await activateAgent());
    } catch (error) {
      fail((error as Error).message);
    }
  };

  /** tell posts one command and reports a refusal without ending the conversation. */
  const tell = (command: Parameters<typeof sendAgentCommand>[0]) => {
    sendAgentCommand(command).catch((error: Error) => setMessage(error.message));
  };

  /**
   * startOver ends the conversation and opens a fresh one: the reader's one explicit way to
   * start again, and the only thing a mount never does. The phase goes through `loading`
   * because that is what restarts the stream effect, which then joins the new conversation.
   * A refusal is a sentence above the composer rather than a dead panel — the conversation
   * the reader has was never ended, so nothing is lost by staying on it.
   */
  const startOver = async () => {
    setPhase("loading");
    setMessage("");
    try {
      const next = await newAgentChat();
      setEnvelope(null);
      setSession("");
      apply(next);
    } catch (error) {
      setPhase("active");
      setMessage((error as Error).message);
    }
  };

  // The width is read once, on mount, and written at the end of a drag or a keyboard step
  // rather than on every pointer move: a preference is a decision, and the pointer passing
  // over the edge is not one.
  // The pet is a browser preference, read once on mount and written on the reader's own
  // acts — the same shape as the width, and never a project file.
  const [petOn, setPetOn] = useState(readPetEnabled);
  const [petVariant, setPetVariant] = useState(readPetVariant);
  const theme = useDocumentTheme();
  // The top bar's control sits behind the panel's own backdrop, so a reader who
  // wants to see the pet in the other theme would close the surface they are
  // checking. The panel carries the same control, with the same cycle.
  const themeState = useThemeOptional();
  const effectivePet = effectiveVariant(petVariant, theme);

  const togglePet = () => {
    const next = !petOn;
    setPetOn(next);
    writePetEnabled(next);
  };

  const choosePet = (variant: PetVariant) => {
    setPetVariant(variant);
    writePetVariant(variant);
  };

  const [width, setWidth] = useState(readPanelWidth);
  const [petHeight, setPetHeight] = useState(readPetHeight);

  // One derivation for the band and the word: the panel owns the machine, because the state
  // must stay readable while the pet itself is off. The signal is the thread's own facts,
  // so a frame that only repeated itself costs no state change.
  const running = Boolean(envelope?.thread.running);
  const signal = useMemo(
    () => petSignal(phase, envelope?.thread, running),
    [phase, envelope, running],
  );
  const petState = usePetState(signal, petTimingFor(effectivePet));

  const dragPanel = (next: number) => setWidth(clampPanelWidth(next));

  const commitPanel = (next: number) => {
    const clamped = clampPanelWidth(next);
    setWidth(clamped);
    writePanelWidth(clamped);
  };

  const dragPet = (next: number) => setPetHeight(clampPetHeight(next));

  const commitPet = (next: number) => {
    const clamped = clampPetHeight(next);
    setPetHeight(clamped);
    writePetHeight(clamped);
  };

  /**
   * sentPrompts is the composer's history: the reader's own messages, in the order the
   * bridge wrote them. It is derived from the thread rather than kept in a list of its own,
   * so it survives a closed panel and cannot disagree with what was actually sent.
   */
  const sentPrompts = useMemo(() => {
    const messages = envelope?.thread.messages ?? [];
    return messages
      .filter((entry) => entry.role === ROLE_USER)
      .map((entry) =>
        entry.parts
          .filter((part) => part.kind === PART_TEXT)
          .map((part) => part.text ?? "")
          .join(""),
      )
      .filter((text) => text.trim() !== "");
  }, [envelope]);

  /**
   * sendPrompt is the runtime's send path: it posts the one prompt command and, on a
   * refusal, puts the sentence on the panel and throws — the runtime turns that throw into
   * a `MessageNotSentError` and restores the draft it cleared, so a refused send never
   * costs the reader the prompt they wrote.
   */
  const sendPrompt = async (text: string) => {
    setMessage("");
    try {
      await sendAgentCommand(promptCommand(text, session));
    } catch (error) {
      setMessage((error as Error).message);
      throw error;
    }
  };

  const command = state?.command ?? [];
  const configured = command.length > 0;

  // Derived during render from the thread, which is the only truth about the model in use: the
  // control reads the agent's own answer, so an accepted choice moves it when the agent says so
  // and a refused one does not move it at all. What it may offer is the project's business:
  // the agent's choices narrowed to the models `.sdt.yaml` declares, in the declared order.
  // An agent that offers no model option gets no control — the panel it had before, and not a
  // disabled box a reader would try.
  const model = modelOption(envelope?.thread.config);
  const offered = useMemo(
    // The value in use is the agent's own, from the option it advertised: the posture can
    // lag it after a switch, and the control must show what the thread says.
    () => (model ? offeredModels(model, state?.models, model.currentValue) : undefined),
    [model, state],
  );
  const modelChoices: UiComboOption[] = (offered?.rows ?? []).map((choice) => ({
    id: choice.value,
    label: choice.name ?? choice.value,
  }));

  return (
    // The entry lives in the top bar rather than inside the panel, so the modal is
    // controlled directly: `open` is the panel's own state, which is what makes
    // "closed means nothing happens" a property of the panel and not of a trigger.
    // A press on the backdrop dismisses it — the same act as Escape — and it is the
    // *overlay* that carries both the class and the prop: react-aria measures "outside"
    // against the element that holds the dialog, so a backdrop styled on that element
    // would be inside it and the press would land on nothing. Dismissing only closes the
    // surface: the agent, the session and a turn in flight are the bridge's and carry on.
    <ModalOverlay
      className="agent-panel__overlay"
      isOpen={open}
      onOpenChange={onOpenChange}
      isDismissable
    >
      <Modal>
        <Dialog
          className="agent-panel"
          aria-label="Agent chat"
          style={
            {
              "--agent-panel-w": `${width}px`,
              "--agent-pet-h": `${petHeight}px`,
            } as CSSProperties
          }
        >
          {({ close }) => (
            <>
              <PanelResizeHandle width={width} onDrag={dragPanel} onCommit={commitPanel} />
              <header className="agent-panel__header">
                <h2 className="agent-panel__title">Agent</h2>
                {/* The pet is a preference; the state is information. With the pet off the
                    word moves here, from the one machine that also drives the band. */}
                {!petOn && (
                  <AgentStateWord
                    phase={phase}
                    state={petState}
                    className="agent-panel__state-word--header"
                  />
                )}
                <div className="agent-panel__header-actions">
                  {/* One explicit way to start over: a mount never ends a conversation, and
                      neither does closing the panel. */}
                  {phase === "active" && (
                    <TooltipButton
                      className="agent-panel__icon-button"
                      label="Start a new chat"
                      tooltip="New chat"
                      onPress={() => void startOver()}
                    >
                      <Icon name="add_comment" />
                    </TooltipButton>
                  )}
                  {/* The same control as the top bar's, repeated here because the modal's
                      backdrop stands between a reader and that one. */}
                  {themeState && (
                    <TooltipButton
                      className="agent-panel__icon-button"
                      label={`Theme: ${THEME_LABEL(themeState.pref)}`}
                      tooltip={`Theme: ${THEME_LABEL(themeState.pref)} — click to cycle`}
                      onPress={themeState.cycle}
                    >
                      <Icon name={THEME_ICON[themeState.pref]} />
                    </TooltipButton>
                  )}
                  {/* The style menu exists only while there is a pet to style; the toggle is
                      always here, because the band it controls collapses to nothing. */}
                  {petOn && (
                    <AgentPetMenu variant={petVariant} theme={theme} onSelect={choosePet} />
                  )}
                  <TooltipButton
                    className="agent-panel__icon-button"
                    label={petOn ? "Hide the pet" : "Show the pet"}
                    tooltip={petOn ? "Hide the pet" : "Show the pet"}
                    aria-pressed={petOn}
                    onPress={togglePet}
                  >
                    <Icon name="pets" />
                  </TooltipButton>
                  <TooltipButton
                    className="agent-panel__icon-button"
                    label="Close the agent panel"
                    tooltip="Close"
                    onPress={close}
                  >
                    <Icon name="close" />
                  </TooltipButton>
                </div>
              </header>
              {/* The pet sits directly under the header, above the conversation, and its
                  lower edge is the height control: the reader's band, with the state word
                  it already carries. Both are absent while the pet is off, so no empty
                  strip is left behind. */}
              {petOn && (
                <>
                  <Suspense fallback={null}>
                    <PetStrip
                      phase={phase}
                      signal={signal}
                      state={petState}
                      variant={effectivePet}
                      family={is2DVariant(effectivePet) ? "2d" : "webgl"}
                      theme={theme}
                    />
                  </Suspense>
                  <PetBandResizeHandle height={petHeight} onDrag={dragPet} onCommit={commitPet} />
                </>
              )}
              <div
                className={`agent-panel__body${
                  phase === "active" && envelope ? " agent-panel__body--thread" : ""
                }`}
              >
                {phase === "loading" && <p className="agent-panel__note">Reading the bridge…</p>}

                {phase === "error" && (
                  <div className="agent-panel__activation">
                    <p className="agent-panel__note agent-panel__note--error">{message}</p>
                    {configured ? (
                      <button
                        type="button"
                        className="agent-panel__action agent-panel__action--secondary"
                        onClick={() => void start()}
                      >
                        Start the agent again
                      </button>
                    ) : (
                      <button
                        type="button"
                        className="agent-panel__action agent-panel__action--secondary"
                        onClick={() => void readState()}
                      >
                        Read the bridge state again
                      </button>
                    )}
                  </div>
                )}

                {phase === "stopped" && (
                  <div className="agent-panel__activation">
                    {configured ? (
                      <>
                        <p className="agent-panel__note">
                          {message === "" ? "The agent is not running." : message}
                        </p>
                        <button
                          type="button"
                          className="agent-panel__action"
                          onClick={() => void start()}
                        >
                          Start the agent
                        </button>
                      </>
                    ) : (
                      <>
                        <p className="agent-panel__note">
                          This project declares no agent. Set <code>agent.command</code> in{" "}
                          <code>.sdt.yaml</code> and reopen this panel.
                        </p>
                        <button
                          type="button"
                          className="agent-panel__action agent-panel__action--secondary"
                          onClick={() => void readState()}
                        >
                          Read the bridge state again
                        </button>
                      </>
                    )}
                  </div>
                )}

                {phase === "starting" && (
                  <p className="agent-panel__note">Starting the agent the project declares…</p>
                )}

                {phase === "restarting" && (
                  <p className="agent-panel__note">
                    {message === ""
                      ? "The agent exited; starting it again…"
                      : `${message} — starting it again…`}
                  </p>
                )}

                {phase === "active" && (
                  // The thread renders from the first active frame: an empty one is the
                  // converter's own state, so the composer is usable before the bridge has
                  // sent anything and the reader is never met by a dead panel.
                  <AgentThread
                    thread={envelope?.thread ?? { sessionId: session, messages: [] }}
                    onAnswer={(requestId, optionId) =>
                      tell(permissionCommand(requestId, optionId, session))
                    }
                    onElicitation={(requestId, action, content) =>
                      tell(elicitationCommand(requestId, action, content, session))
                    }
                    onSend={sendPrompt}
                    onCancel={() => tell(cancelCommand(session))}
                  >
                    {/* The footer reads the same state the panel does, and it lives inside
                        the runtime because the composer is a runtime primitive now. */}
                    {message !== "" && (
                      <p className="agent-panel__note agent-panel__note--error">{message}</p>
                    )}
                    {model && modelChoices.length > 0 && (
                      <div className="agent-panel__model">
                        <ComboBox
                          ariaLabel="Model"
                          label="Model"
                          options={modelChoices}
                          selectedKey={model.currentValue ?? null}
                          onSelectionChange={(key) => {
                            // The choice is sent and the control waits for the agent to confirm
                            // it: nothing moves here until the thread says the agent took it, and
                            // a refusal leaves the sentence above this control and the model
                            // unchanged.
                            if (typeof key !== "string" || key === model.currentValue) return;
                            tell(configCommand(model.id, key, session));
                          }}
                        />
                      </div>
                    )}
                    {/* The declaration named nothing this agent offers — a stale id, or a list
                        written for another agent. No picker instead of an empty one, and the
                        model the session is on, so the reader is not left guessing either. */}
                    {offered?.declared && !offered.matched && (
                      <div className="agent-panel__model">
                        <p className="agent-panel__note agent-panel__model-note">
                          No model this agent offers is one this project declares
                          {model?.currentValue ? ` — the session is on ${model.currentValue}` : ""}.
                        </p>
                      </div>
                    )}
                    <AgentComposer
                      sentPrompts={sentPrompts}
                      commands={envelope?.thread.commands ?? []}
                    />
                  </AgentThread>
                )}
              </div>
            </>
          )}
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}

/**
 * PanelResizeHandle is the panel's inline-start edge as a control: a pointer drag and the
 * keyboard on one focusable separator, which is the ARIA window-splitter pattern.
 *
 * It is a plain element rather than react-aria's `Separator`: that primitive is a static
 * divider whose props carry no `tabIndex`, no `aria-value*` and no keyboard handler, so it
 * cannot express a splitter — see the task file's Deviations.
 */
function PanelResizeHandle({
  width,
  onDrag,
  onCommit,
}: {
  width: number;
  onDrag: (next: number) => void;
  onCommit: (next: number) => void;
}) {
  // The drag's origin, held across pointer moves: a move reports a position, not a delta.
  const drag = useRef<{ x: number; width: number } | null>(null);
  const [resizing, setResizing] = useState(false);

  /** widthAt is the width a pointer's x asks for; the panel is anchored to the inline end,
   *  so moving the handle left — a smaller x — grows it. */
  const widthAt = (clientX: number) => {
    const start = drag.current;
    return start ? clampPanelWidth(start.width + (start.x - clientX)) : width;
  };

  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label="Resize the agent panel"
      aria-valuemin={MIN_PANEL_WIDTH}
      aria-valuemax={MAX_PANEL_WIDTH}
      aria-valuenow={width}
      tabIndex={0}
      className="agent-panel__resize"
      data-resizing={resizing ? "" : undefined}
      onPointerDown={(event) => {
        drag.current = { x: event.clientX, width };
        setResizing(true);
        event.currentTarget.setPointerCapture(event.pointerId);
      }}
      onPointerMove={(event) => {
        if (drag.current) onDrag(widthAt(event.clientX));
      }}
      onPointerUp={(event) => {
        if (!drag.current) return;
        const next = widthAt(event.clientX);
        drag.current = null;
        setResizing(false);
        onCommit(next);
      }}
      onPointerCancel={() => {
        drag.current = null;
        setResizing(false);
      }}
      onKeyDown={(event) => {
        // The handle's own keys, the other half of the same control: a step, a finer step,
        // and the way back to what the panel opens at.
        const step = event.shiftKey ? 8 : 24;
        const next =
          event.key === "ArrowLeft"
            ? width + step
            : event.key === "ArrowRight"
              ? width - step
              : event.key === "Home"
                ? DEFAULT_PANEL_WIDTH
                : null;
        if (next === null) return;
        event.preventDefault();
        onCommit(next);
      }}
    />
  );
}

/**
 * PetBandResizeHandle is the pet band's lower edge as a control: a pointer drag and the
 * keyboard on one focusable separator, on the other axis of the same ARIA window-splitter
 * pattern `PanelResizeHandle` uses.
 *
 * Dragging downward opens the band toward the conversation; the preference is written at the
 * end of a gesture, not on every move.
 */
function PetBandResizeHandle({
  height,
  onDrag,
  onCommit,
}: {
  height: number;
  onDrag: (next: number) => void;
  onCommit: (next: number) => void;
}) {
  // The drag's origin, held across pointer moves: a move reports a position, not a delta.
  const drag = useRef<{ y: number; height: number } | null>(null);
  const [resizing, setResizing] = useState(false);

  /** heightAt is the height a pointer's y asks for: the band grows downward. */
  const heightAt = (clientY: number) => {
    const start = drag.current;
    return start ? clampPetHeight(start.height + (clientY - start.y)) : height;
  };

  return (
    <div
      role="separator"
      aria-orientation="horizontal"
      aria-label="Resize the pet band"
      aria-valuemin={MIN_PET_HEIGHT}
      aria-valuemax={MAX_PET_HEIGHT}
      aria-valuenow={height}
      tabIndex={0}
      className="agent-panel__pet-resize"
      data-resizing={resizing ? "" : undefined}
      onPointerDown={(event) => {
        drag.current = { y: event.clientY, height };
        setResizing(true);
        event.currentTarget.setPointerCapture(event.pointerId);
      }}
      onPointerMove={(event) => {
        if (drag.current) onDrag(heightAt(event.clientY));
      }}
      onPointerUp={(event) => {
        if (!drag.current) return;
        const next = heightAt(event.clientY);
        drag.current = null;
        setResizing(false);
        onCommit(next);
      }}
      onPointerCancel={() => {
        drag.current = null;
        setResizing(false);
      }}
      onKeyDown={(event) => {
        // The width handle's vocabulary on the vertical axis: a step, a finer step, and the
        // way back to the height the band opens at.
        const step = event.shiftKey ? 8 : 24;
        const next =
          event.key === "ArrowDown"
            ? height + step
            : event.key === "ArrowUp"
              ? height - step
              : event.key === "Home"
                ? DEFAULT_PET_HEIGHT
                : null;
        if (next === null) return;
        event.preventDefault();
        onCommit(next);
      }}
    />
  );
}

/** The pet's six names, in the reader's words; the vocabulary itself is the library's. */
const PET_VARIANT_LABELS: Record<PetVariant, string> = {
  dot: "Dot",
  sciame: "Sciame",
  trama: "Trama",
  fiamma: "Fiamma",
  scriptor: "Scriptor",
  possibile: "Possibile",
};

/**
 * AgentPetMenu is the pet's style picker in the panel header.
 *
 * The additive variants are disabled on the light theme rather than hidden: a reader who
 * wonders where they went reads the sentence under the list. The control is absent while
 * the pet is off — there is nothing to style — and the toggle beside it is always there.
 */
function AgentPetMenu({
  variant,
  theme,
  onSelect,
}: {
  variant: PetVariant;
  theme: "light" | "dark";
  onSelect: (variant: PetVariant) => void;
}) {
  const light = theme === "light";
  return (
    <MenuTrigger>
      <Button className="agent-panel__icon-button" aria-label="Pet style">
        <Icon name="style" />
      </Button>
      <Popover className="agent-panel__pet-popover" placement="bottom end" offset={4}>
        <Menu
          className="agent-panel__pet-menu"
          aria-label="Pet style"
          selectionMode="single"
          selectedKeys={[effectiveVariant(variant, theme)]}
          disabledKeys={light ? ["sciame", "trama"] : []}
          onAction={(key) => onSelect(key as PetVariant)}
        >
          {PET_VARIANTS.map((choice) => (
            <MenuItem key={choice} id={choice} className="agent-panel__pet-item">
              {PET_VARIANT_LABELS[choice]}
            </MenuItem>
          ))}
        </Menu>
        {light ? (
          <p className="agent-panel__note agent-panel__pet-note">
            Sciame and trama are additive: they need the dark theme.
          </p>
        ) : null}
      </Popover>
    </MenuTrigger>
  );
}

/**
 * AgentComposer is the composer as the runtime's own primitive: the runtime owns the text,
 * the send and the stop, so the panel has one send path instead of two, and the send it
 * refuses restores the draft through the runtime's own error path.
 *
 * What stays here is the reading vocabulary the panel already had — Enter is a newline and
 * Cmd/Ctrl+Enter sends (`submitMode`), the input grows to eight lines and then scrolls
 * (`maxRows`), the arrows walk the prompts already sent (`composerHistory`) — plus the `/`
 * palette over the commands the agent advertises: the bridge folds them already
 * (`thread.commands`), and nothing rendered them.
 */
function AgentComposer({
  sentPrompts,
  commands,
}: {
  sentPrompts: string[];
  commands: AgentCommand[];
}) {
  const aui = useAui();
  const input = useRef<HTMLTextAreaElement | null>(null);
  // Where the reader is in the prompts already sent; the texts themselves are the thread's.
  const [walk, setWalk] = useState<ComposerWalk>(NO_WALK);
  // A history that changed — a prompt sent, a conversation started over — puts the walk back
  // at the newest entry. Derived during render, React's own pattern for a reset.
  const [history, setHistory] = useState(sentPrompts);
  if (history !== sentPrompts) {
    setHistory(sentPrompts);
    setWalk(NO_WALK);
  }

  // The adapter is the agent's commands as the popover wants them: one category, because a
  // slash command has no second dimension, and the same list for a search.
  const adapter = useMemo(
    () => ({
      categories: () => (commands.length === 0 ? [] : [{ id: "commands", label: "Commands" }]),
      categoryItems: () => commandItems(commands),
      search: () => commandItems(commands),
    }),
    [commands],
  );

  /**
   * onKeyDown is the composer's vocabulary. While the `/` palette is open it owns the
   * arrows — the input says so itself, with the `aria-expanded` the primitive writes — so
   * the history is only the fallback, never a second owner of the same key.
   */
  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.currentTarget.getAttribute("aria-expanded") === "true") return;
    if (event.nativeEvent.isComposing) return;
    const direction: -1 | 1 | null =
      event.key === "ArrowUp" ? -1 : event.key === "ArrowDown" ? 1 : null;
    if (direction === null) return;
    const field = event.currentTarget;
    if (field.selectionStart !== field.selectionEnd) return;
    const draft = field.value;
    if (!caretAtEdge(draft, field.selectionStart, direction)) return;
    const next = recall(sentPrompts, walk, direction, draft);
    if (!next) return;
    event.preventDefault();
    aui.composer.setText(next.draft);
    setWalk(next.walk);
    // The caret goes after what was recalled, once the new text is in the field.
    requestAnimationFrame(() => {
      const el = input.current;
      if (!el) return;
      const end = el.value.length;
      el.setSelectionRange(end, end);
    });
  };

  return (
    <ComposerPrimitive.TriggerPopoverRoot>
      <ComposerPrimitive.Root className="agent-panel__composer">
        <ComposerPrimitive.Input
          ref={input}
          rows={1}
          minRows={1}
          maxRows={8}
          submitMode="ctrlEnter"
          placeholder="Ask the agent…"
          aria-label="Message"
          onKeyDown={onKeyDown}
        />
        <ComposerPrimitive.Send className="agent-panel__action">Send</ComposerPrimitive.Send>
        <ComposerPrimitive.Cancel className="agent-panel__action agent-panel__action--quiet">
          Stop
        </ComposerPrimitive.Cancel>
        {/* Only when the agent advertises a command: a palette with nothing in it is the
            control that looks live and does nothing. */}
        {commands.length > 0 ? (
          <ComposerPrimitive.TriggerPopover
            char="/"
            adapter={adapter}
            className="agent-panel__commands"
          >
            <ComposerPrimitive.TriggerPopover.Directive />
            <ComposerPrimitive.TriggerPopoverCategories>
              {(categories) =>
                categories.map((category) => (
                  <ComposerPrimitive.TriggerPopoverCategoryItem
                    key={category.id}
                    categoryId={category.id}
                    className="agent-panel__command"
                  >
                    {category.label}
                  </ComposerPrimitive.TriggerPopoverCategoryItem>
                ))
              }
            </ComposerPrimitive.TriggerPopoverCategories>
            <ComposerPrimitive.TriggerPopoverItems>
              {(items) =>
                items.map((item, index) => (
                  <ComposerPrimitive.TriggerPopoverItem
                    key={item.id}
                    item={item}
                    index={index}
                    className="agent-panel__command"
                  >
                    <span className="agent-panel__command-name">{item.label}</span>
                    {item.description ? (
                      <span className="agent-panel__command-note">{item.description}</span>
                    ) : null}
                  </ComposerPrimitive.TriggerPopoverItem>
                ))
              }
            </ComposerPrimitive.TriggerPopoverItems>
          </ComposerPrimitive.TriggerPopover>
        ) : null}
      </ComposerPrimitive.Root>
    </ComposerPrimitive.TriggerPopoverRoot>
  );
}

/** commandItems is the agent's commands as the popover's own item shape. */
function commandItems(commands: AgentCommand[]) {
  return commands.map((command) => ({
    id: command.name,
    type: "command",
    label: command.name,
    description: command.description,
  }));
}
