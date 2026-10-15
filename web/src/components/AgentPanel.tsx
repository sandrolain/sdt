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
import { Dialog, DialogTrigger, Modal } from "react-aria-components";
import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  activateAgent,
  connectAgent,
  fetchAgentState,
  permissionCommand,
  promptCommand,
  cancelCommand,
  type AgentConnection,
  type AgentState,
  type AgentStateEnvelope,
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
  const connection = useRef<AgentConnection | null>(null);

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

  // One socket for the page, opened when the bridge is active and closed when it is
  // not: a conversation lives as long as the viewer does, not as long as the panel is
  // on screen, so reopening the panel shows the thread as it stands.
  useEffect(() => {
    if (phase !== "active") return;
    const opened = connectAgent({
      onState: setEnvelope,
      onClose: (reason) => {
        setMessage(reason);
        setPhase("error");
      },
    });
    connection.current = opened;
    return () => {
      opened.close();
      connection.current = null;
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

  const onPrompt = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    const text = new FormData(form).get("prompt");
    if (typeof text !== "string" || text.trim() === "") return;
    connection.current?.send(promptCommand(text));
    form.reset();
  };

  return (
    <DialogTrigger isOpen={open} onOpenChange={onOpenChange}>
      {/* The trigger is the top bar's entry; the panel never opens itself. */}
      <button
        type="button"
        className="agent-panel__opener"
        aria-label="Chat with an agent"
        hidden
      />
      <Modal className="agent-panel__overlay">
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
                      The bridge is off. Start a proxy in front of your agent and give it the
                      socket:
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
                  <AgentThreadView
                    envelope={envelope}
                    onAnswer={(requestId, allow) =>
                      connection.current?.send(permissionCommand(requestId, allow))
                    }
                  />
                )}
                {phase === "active" && !envelope && (
                  <p className="agent-panel__note">Connected. Waiting for the agent…</p>
                )}
              </div>
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
                    onClick={() => connection.current?.send(cancelCommand())}
                  >
                    Stop
                  </button>
                </form>
              )}
            </>
          )}
        </Dialog>
      </Modal>
    </DialogTrigger>
  );
}

/**
 * AgentThreadView is the rendering seam: the assistant-ui runtime takes this over
 * (its messages and parts are the same model), and until it does the panel shows the
 * parts the bridge sent, so the activation flow is verifiable end to end today.
 */
export function AgentThreadView({
  envelope,
  onAnswer,
}: {
  envelope: AgentStateEnvelope;
  onAnswer: (requestId: string, allow: boolean) => void;
}) {
  return (
    <div className="agent-panel__thread">
      {envelope.thread.messages.map((message) => (
        <article
          key={message.id}
          className={`agent-panel__message agent-panel__message--${message.role}`}
        >
          {message.parts.map((part, index) =>
            part.permission ? (
              <PermissionCard key={index} permission={part.permission} onAnswer={onAnswer} />
            ) : (
              <p key={index} className="agent-panel__part">
                {partLabel(part)}
              </p>
            ),
          )}
        </article>
      ))}
      {envelope.thread.plan?.entries?.length ? (
        <ol className="agent-panel__plan">
          {envelope.thread.plan.entries.map((entry) => (
            <li key={entry.content}>
              {entry.content}
              {entry.status ? ` (${entry.status})` : ""}
            </li>
          ))}
        </ol>
      ) : null}
    </div>
  );
}

/**
 * PermissionCard is the approve/deny card: nothing runs unasked, so the agent blocks
 * here until the user answers, and the answer replaces the buttons with the decision
 * the bridge records — the card resolves where it stands rather than moving.
 */
export function PermissionCard({
  permission,
  onAnswer,
}: {
  permission: { requestId: string; title?: string; toolKind?: string; decision?: string };
  onAnswer: (requestId: string, allow: boolean) => void;
}) {
  return (
    <div className="agent-panel__card">
      <p className="agent-panel__part">
        {permission.title ?? "The agent asks for permission"}
        {permission.toolKind ? ` (${permission.toolKind})` : ""}
      </p>
      {permission.decision ? (
        <p className="agent-panel__part agent-panel__decision">{permission.decision}</p>
      ) : (
        <div className="agent-panel__card-actions">
          <button
            type="button"
            className="agent-panel__action"
            onClick={() => onAnswer(permission.requestId, true)}
          >
            Allow
          </button>
          <button
            type="button"
            className="agent-panel__action"
            onClick={() => onAnswer(permission.requestId, false)}
          >
            Deny
          </button>
        </div>
      )}
    </div>
  );
}

/** partLabel is one rendered line per part, whatever the variant produced. */
function partLabel(part: {
  kind: string;
  text?: string;
  toolCall?: { title?: string };
  permission?: { title?: string; decision?: string };
}): string {
  if (part.text) return part.text;
  if (part.toolCall) return `Tool: ${part.toolCall.title ?? "a tool"}`;
  if (part.permission) {
    return `Permission: ${part.permission.title ?? "a tool"}${part.permission.decision ? ` — ${part.permission.decision}` : ""}`;
  }
  return part.kind;
}
