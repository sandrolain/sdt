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
import { Dialog, Modal, ModalOverlay } from "react-aria-components";
import { AgentThread } from "./AgentThread";
import { ComboBox, type UiComboOption } from "./ui/ComboBox";
import { TooltipButton } from "./ui/Tooltip";
import { Icon } from "../lib/icon";
import { useEffect, useRef, useState, type CSSProperties, type FormEvent } from "react";
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
  permissionCommand,
  promptCommand,
  sendAgentCommand,
  streamAgent,
  type AgentState,
  type AgentStateEnvelope,
  type AgentStatusFrame,
  type AgentStream,
} from "../lib/agent";

/** What the panel is showing, as one value rather than a set of booleans. */
type Phase = "loading" | "starting" | "active" | "restarting" | "stopped" | "error";

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

  // Opening is a change of props, not an event of ours, so the reset happens during
  // render — React's documented pattern — rather than in an effect that would only
  // derive state.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      setPhase("loading");
      setMessage("");
    }
  }

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

  // The width is read once, on mount, and written at the end of a drag or a keyboard step
  // rather than on every pointer move: a preference is a decision, and the pointer passing
  // over the edge is not one.
  const [width, setWidth] = useState(readPanelWidth);

  const dragPanel = (next: number) => setWidth(clampPanelWidth(next));

  const commitPanel = (next: number) => {
    const clamped = clampPanelWidth(next);
    setWidth(clamped);
    writePanelWidth(clamped);
  };

  const onPrompt = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    const text = new FormData(form).get("prompt");
    if (typeof text !== "string" || text.trim() === "") return;
    tell(promptCommand(text, session));
    form.reset();
  };

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
                <TooltipButton
                  className="agent-panel__icon-button"
                  label="Close the agent panel"
                  tooltip="Close"
                  onPress={close}
                >
                  <Icon name="close" />
                </TooltipButton>
              </header>
              <div className="agent-panel__body">
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
                  />
                )}
                {phase === "active" && !envelope && (
                  <p className="agent-panel__note">Connected. Waiting for the agent…</p>
                )}
              </div>
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
                  <input
                    type="text"
                    name="prompt"
                    placeholder="Ask the agent…"
                    autoComplete="off"
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
