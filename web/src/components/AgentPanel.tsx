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
 * steps it, and the value is remembered across sessions.
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
import { useDocumentTheme } from "../lib/themeAttr";
import {
  Suspense,
  lazy,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type FormEvent,
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
  activateAgent,
  cancelCommand,
  configCommand,
  fetchAgentState,
  modelOption,
  newAgentChat,
  elicitationCommand,
  permissionCommand,
  promptCommand,
  sendAgentCommand,
  streamAgent,
  type AgentState,
  type AgentStateEnvelope,
  type AgentStatusFrame,
  type AgentStream,
} from "../lib/agent";

/**
 * The pet is lazy for the same reason the canvas is: its library imports `three`, and a
 * reader who never opens the panel never downloads it. The band declares its own height, so
 * the chunk arriving never resizes the thread.
 */
const PetStrip = lazy(() => import("./PetStrip"));

/** What the panel is showing, as one value rather than a set of booleans. */
type Phase = "loading" | "starting" | "active" | "restarting" | "stopped" | "error";

/** The composer's ceiling before it scrolls inside: roughly eight lines of the panel's type. */
const COMPOSER_MAX_HEIGHT = 176;

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
  const [draft, setDraft] = useState("");
  const composer = useRef<HTMLTextAreaElement>(null);
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

  // One derivation for the band and the word: the panel owns the machine, because the state
  // must stay readable while the pet itself is off. The signal is the thread's own facts,
  // so a frame that only repeated itself costs no state change.
  const running = Boolean(envelope?.thread.running);
  const signal = useMemo(
    () => petSignal(phase, envelope?.thread, running),
    [phase, envelope, running],
  );
  const petState = usePetState(signal);

  const dragPanel = (next: number) => setWidth(clampPanelWidth(next));

  const commitPanel = (next: number) => {
    const clamped = clampPanelWidth(next);
    setWidth(clamped);
    writePanelWidth(clamped);
  };

  const onPrompt = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (draft.trim() === "") return;
    // The draft clears only once the viewer accepted it: a refused send must not cost the
    // reader the prompt they just wrote.
    sendAgentCommand(promptCommand(draft, session))
      .then(() => {
        setDraft("");
        setMessage("");
      })
      .catch((error: Error) => setMessage(error.message));
  };

  /** The composer's chord: Enter is a newline, Cmd/Ctrl+Enter is the send. */
  const onComposerKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== "Enter" || (!event.metaKey && !event.ctrlKey)) return;
    event.preventDefault();
    event.currentTarget.form?.requestSubmit();
  };

  // The composer grows with its draft up to a cap and then scrolls inside. The height is
  // measured because `field-sizing: content` is not available in every browser this app
  // runs in; the effect writes style, never state, so it cannot re-render on its own. jsdom
  // reports no layout, and the guard leaves the element alone there.
  useLayoutEffect(() => {
    const el = composer.current;
    if (!el) return;
    el.style.height = "auto";
    if (el.scrollHeight === 0) return;
    el.style.height = `${Math.min(el.scrollHeight, COMPOSER_MAX_HEIGHT)}px`;
    el.style.overflowY = el.scrollHeight > COMPOSER_MAX_HEIGHT ? "auto" : "hidden";
  }, [draft]);

  const command = state?.command ?? [];
  const configured = command.length > 0;

  // Derived during render from the thread, which is the only truth about the model in use: the
  // control reads the agent's own answer, so an accepted choice moves it when the agent says so
  // and a refused one does not move it at all. An agent that offers no model option gets no
  // control — the panel it had before, and not a disabled box a reader would try.
  const model = modelOption(envelope?.thread.config);
  const modelChoices: UiComboOption[] = (model?.options ?? []).map((choice) => ({
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
          style={{ "--agent-panel-w": `${width}px` } as CSSProperties}
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
                        <code className="agent-panel__recipe">{command.join(" ")}</code>
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

                {phase === "active" && envelope && (
                  <AgentThread
                    thread={envelope.thread}
                    onAnswer={(requestId, allow) =>
                      tell(permissionCommand(requestId, allow, session))
                    }
                    onElicitation={(requestId, action, content) =>
                      tell(elicitationCommand(requestId, action, content, session))
                    }
                  />
                )}
                {phase === "active" && !envelope && (
                  <p className="agent-panel__note">Connected. Waiting for the agent…</p>
                )}
              </div>
              {petOn && (
                <Suspense fallback={null}>
                  <PetStrip
                    phase={phase}
                    signal={signal}
                    state={petState}
                    variant={effectiveVariant(petVariant, theme)}
                  />
                </Suspense>
              )}
              {phase === "active" && message !== "" && (
                <p className="agent-panel__note agent-panel__note--error">{message}</p>
              )}
              {phase === "active" && model && (
                <div className="agent-panel__model">
                  <ComboBox
                    ariaLabel="Model"
                    label="Model"
                    options={modelChoices}
                    selectedKey={model.currentValue ?? null}
                    onSelectionChange={(key) => {
                      // The choice is sent and the control waits for the agent to confirm it:
                      // nothing moves here until the thread says the agent took it, and a
                      // refusal leaves the sentence above this control and the model unchanged.
                      if (typeof key !== "string" || key === model.currentValue) return;
                      tell(configCommand(model.id, key, session));
                    }}
                  />
                </div>
              )}
              {phase === "active" && (
                <form className="agent-panel__composer" onSubmit={onPrompt}>
                  <textarea
                    ref={composer}
                    rows={1}
                    name="prompt"
                    value={draft}
                    onChange={(event) => setDraft(event.target.value)}
                    onKeyDown={onComposerKeyDown}
                    placeholder="Ask the agent…"
                    aria-label="Message"
                  />
                  <button type="submit" className="agent-panel__action">
                    Send
                  </button>
                  <button
                    type="button"
                    className="agent-panel__action agent-panel__action--quiet"
                    onClick={() => tell(cancelCommand(session))}
                  >
                    Stop
                  </button>
                </form>
              )}
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

/** The pet's three names, in the reader's words; the vocabulary itself is the library's. */
const PET_VARIANT_LABELS: Record<PetVariant, string> = {
  dot: "Dot",
  sciame: "Sciame",
  trama: "Trama",
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
