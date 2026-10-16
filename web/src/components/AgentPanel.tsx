/**
 * AgentPanel — the agent bridge as a left slide-in panel over any screen.
 *
 * The panel is the app's own overlay idiom (`Modal` + `Dialog`, the pair
 * `SearchPalette` and `GraphGuide` already use) rather than a route, so a reader can
 * watch the map or read a document and talk to the agent beside it. Dockview's
 * floating groups were considered and rejected: they exist only inside a dockview
 * container, and this panel has to be available wherever the reader is.
 *
 * It is **inert until it is first opened**: closed, it renders nothing, asks nothing
 * and holds no socket. Opening it reads the bridge's state — the only endpoint
 * reachable before activation — and only a submit of the socket path dials. The path
 * is never remembered, never pre-filled and never defaulted to.
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

/** The proxy the user runs in front of their agent. `opencode acp` is the one
 *  verified on this machine; any agent behind a socket works the same way. */
const PROXY_RECIPE = 'socat UNIX-LISTEN:/tmp/sdt-acp.sock,fork EXEC:"opencode acp"';

/** What the panel is doing, as one value rather than three booleans. */
type Phase = "loading" | "inert" | "activating" | "active" | "error";

export function AgentPanel({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [phase, setPhase] = useState<Phase>("loading");
  const [socket, setSocket] = useState("");
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

  const fail = (reason: string) => {
    setMessage(reason);
    setPhase("error");
  };

  const applyState = (state: AgentState) => {
    if (state.error) {
      fail(state.error);
      return;
    }
    setMessage("");
    setPhase(state.activated ? "active" : "inert");
  };

  /** readState re-reads the bridge's posture from the retry button, so it may set the
   *  loading phase itself: it runs from a click, not from an effect. */
  const readState = async () => {
    setPhase("loading");
    try {
      applyState(await fetchAgentState());
    } catch (error) {
      fail((error as Error).message);
    }
  };

  // Opening the panel is what starts the flow: nothing is asked, and nothing is
  // dialled, before it. A stale answer from a previous open is dropped rather than
  // applied.
  useEffect(() => {
    if (!open) return;
    let live = true;
    fetchAgentState()
      .then((state) => {
        if (live) applyState(state);
      })
      .catch((error: Error) => {
        if (live) fail(error.message);
      });
    return () => {
      live = false;
    };
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps -- applyState/fail only set state

  // One stream for the page, opened when the bridge is active and closed when it is
  // not: a conversation lives as long as the viewer does, not as long as the panel is
  // on screen, so reopening the panel shows the thread as it stands.
  useEffect(() => {
    if (phase !== "active") return;
    const opened = streamAgent({
      onState: (next) => {
        setEnvelope(next);
        setSession(next.sessionId);
      },
      onClose: (reason) => {
        setMessage(reason);
        setPhase("error");
      },
    });
    stream.current = opened;
    return () => {
      opened.close();
      stream.current = null;
    };
  }, [phase]);

  const onSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setPhase("activating");
    try {
      applyState(await activateAgent(socket));
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
                  <p className="agent-panel__note">{message}</p>
                  <button
                    type="button"
                    className="agent-panel__action"
                    onClick={() => void readState()}
                  >
                    Read the bridge state again
                  </button>
                </div>
              )}
              {(phase === "inert" || phase === "activating") && (
                <form className="agent-panel__activation" onSubmit={onSubmit}>
                  <p className="agent-panel__note">
                    The bridge is off. Start a proxy in front of your agent and give it the socket:
                  </p>
                  <code className="agent-panel__recipe">{PROXY_RECIPE}</code>
                  <label className="agent-panel__field">
                    <span>Socket path</span>
                    <input
                      type="text"
                      name="socket"
                      value={socket}
                      placeholder="/tmp/sdt-acp.sock"
                      autoComplete="off"
                      spellCheck={false}
                      onChange={(event) => setSocket(event.target.value)}
                    />
                  </label>
                  <button
                    type="submit"
                    className="agent-panel__action"
                    disabled={phase === "activating" || socket.trim() === ""}
                  >
                    {phase === "activating" ? "Connecting…" : "Connect"}
                  </button>
                </form>
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
