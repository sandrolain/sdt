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
 */
import { Dialog, Modal } from "react-aria-components";
import { AgentThread } from "./AgentThread";
import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  activateAgent,
  cancelCommand,
  fetchAgentState,
  permissionCommand,
  promptCommand,
  sendAgentCommand,
  streamAgent,
  type AgentState,
  type AgentStateEnvelope,
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

  // A start and a restart are the bridge's to finish, so the panel asks how they went
  // rather than guessing: while the process is coming up or coming back it re-reads the
  // posture and stops as soon as the answer is one thing or the other. It is the only
  // polling here, and it exists because the stream carries the conversation, not the
  // process.
  useEffect(() => {
    if (phase !== "starting" && phase !== "restarting") return;
    const tick = setInterval(() => {
      fetchAgentState()
        .then(apply)
        .catch((error: Error) => fail(error.message));
    }, 1500);
    return () => clearInterval(tick);
  }, [phase]); // eslint-disable-line react-hooks/exhaustive-deps -- apply/fail only set state

  // The stream carries the conversation, so the panel reconciles its posture slowly while it
  // is up: a restart can finish inside a liveness tick, and when it does the bridge's own
  // sentence — that it could not put the agent back on the conversation it had — is the one
  // thing a reader must not miss.
  useEffect(() => {
    if (phase !== "active") return;
    const tick = setInterval(() => {
      fetchAgentState()
        .then(apply)
        .catch(() => undefined);
    }, 10000);
    return () => clearInterval(tick);
  }, [phase]); // eslint-disable-line react-hooks/exhaustive-deps -- apply only sets state

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

  return (
    // The entry lives in the top bar rather than inside the panel, so the modal is
    // controlled directly: `open` is the panel's own state, which is what makes
    // "closed means nothing happens" a property of the panel and not of a trigger.
    <Modal className="agent-panel__overlay" isOpen={open} onOpenChange={onOpenChange}>
      <Dialog className="agent-panel" aria-label="Agent chat">
        {({ close }) => (
          <>
            <header className="agent-panel__header">
              <h2 className="agent-panel__title">Agent</h2>
              <button
                type="button"
                className="agent-panel__close"
                onClick={close}
                aria-label="Close the agent panel"
              >
                close
              </button>
            </header>
            <div className="agent-panel__body">
              {phase === "loading" && <p className="agent-panel__note">Reading the bridge…</p>}

              {phase === "error" && (
                <div className="agent-panel__activation">
                  <p className="agent-panel__note agent-panel__note--error">{message}</p>
                  {configured ? (
                    <button
                      type="button"
                      className="agent-panel__action"
                      onClick={() => void start()}
                    >
                      Start the agent again
                    </button>
                  ) : (
                    <button
                      type="button"
                      className="agent-panel__action"
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
                        className="agent-panel__action"
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
                  className="agent-panel__action"
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
  );
}
