/**
 * AgentThread — the conversation, rendered with assistant-ui.
 *
 * The runtime is `useExternalStoreRuntime`: the bridge owns the state, and
 * `lib/agentThread.ts` is the hand-written converter that says what each part is. The
 * runtime's own rendering is taken as vendored (the plan's call): this file gives it
 * the thread, overrides the one part it has no opinion about — the permission card,
 * which is a `data-permission` part — and leaves text, reasoning and tool calls to the
 * runtime's defaults.
 *
 * The composer and the card's answer travel as commands, not as runtime payloads: the
 * panel posts them, so the runtime is a view here and the bridge stays the only
 * accumulator.
 */
import {
  AssistantRuntimeProvider,
  MessagePrimitive,
  ThreadPrimitive,
  useExternalStoreRuntime,
} from "@assistant-ui/react";
import type {
  ThreadMessageLike,
  ToolCallMessagePartComponent,
  TextMessagePartComponent,
  ReasoningMessagePartComponent,
} from "@assistant-ui/react";
import { createContext, useContext, useMemo } from "react";
import { threadMessages, type ConvertedMessage } from "../lib/agentThread";
import type { AgentPermission, AgentPlan, AgentThread as AgentThreadState } from "../lib/agent";

/** The answer to a card travels up through a context: a part cannot take a prop. */
const AnswerContext = createContext<(requestId: string, allow: boolean) => void>(() => undefined);

export function AgentThread({
  thread,
  onAnswer,
}: {
  thread: AgentThreadState;
  onAnswer: (requestId: string, allow: boolean) => void;
}) {
  // The converter is where the runtime expects one: `messages` are the bridge's own
  // shape and the runtime normalises them (metadata included) on the way into its store.
  const runtime = useExternalStoreRuntime<ConvertedMessage>({
    messages: useMemo(() => threadMessages(thread), [thread]),
    // The one cast the library's union forces: the converter states its own part shapes and
    // the runtime normalises them on the way into its store.
    convertMessage: (message) => message as unknown as ThreadMessageLike,
    // The panel owns the composer, so the runtime's own send path is not used; it still
    // has to exist, because the adapter has no shape without it.
    onNew: async () => undefined,
    // The bridge's wire has no turn-completion event (the state is the whole thread and
    // nothing else), so the runtime is never told a turn is running; the thread shows
    // what it has rather than a spinner it cannot justify.
    isRunning: false,
  });

  return (
    <AnswerContext.Provider value={onAnswer}>
      <AssistantRuntimeProvider runtime={runtime}>
        <ThreadPrimitive.Root className="agent-thread">
          <ThreadPrimitive.Viewport className="agent-thread__viewport">
            <ThreadPrimitive.Messages
              components={{
                UserMessage: () => <MessageRow role="user" />,
                AssistantMessage: () => <MessageRow role="assistant" />,
              }}
            />
          </ThreadPrimitive.Viewport>
        </ThreadPrimitive.Root>
      </AssistantRuntimeProvider>
    </AnswerContext.Provider>
  );
}

/** MessageRow renders one message; the runtime fills the parts. */
function MessageRow({ role }: { role: "user" | "assistant" }) {
  return (
    <MessagePrimitive.Root className={`agent-panel__message agent-panel__message--${role}`}>
      <MessagePrimitive.Parts
        components={{
          // The runtime's primitives are headless: the styled surface is a package
          // this wave deliberately did not take, so the parts this project renders are
          // written here, with its own tokens.
          Text: TextPart,
          Reasoning: ReasoningPart,
          tools: { Fallback: ToolCallPart },
          data: {
            by_name: {
              permission: PermissionPart,
              plan: PlanPart,
              error: ErrorPart,
              stop: StopPart,
              waiting: WaitingPart,
            },
          },
        }}
      />
    </MessagePrimitive.Root>
  );
}

/** TextPart is what the agent (or the user) said. */
const TextPart: TextMessagePartComponent = ({ text }) => (
  <p className="agent-panel__part">{text}</p>
);

/** ReasoningPart is the agent's thinking: collapsed, because it is not the answer. */
const ReasoningPart: ReasoningMessagePartComponent = ({ text }) => (
  <details className="agent-panel__reasoning">
    <summary>Reasoning</summary>
    <p className="agent-panel__part">{text}</p>
  </details>
);

/** ToolCallPart is one tool the agent ran: what it was given, and what came back. */
const ToolCallPart: ToolCallMessagePartComponent = ({
  toolName,
  argsText,
  result,
  isError,
  status,
}) => (
  <div className={`agent-panel__tool${isError ? " is-error" : ""}`}>
    <p className="agent-panel__part">
      <span className="agent-panel__tool-name">{toolName}</span>
      {status?.type === "running" ? " — running" : ""}
    </p>
    {argsText ? <pre className="agent-panel__tool-args">{argsText}</pre> : null}
    {result === undefined ? null : (
      <pre className="agent-panel__tool-result">
        {typeof result === "string" ? result : JSON.stringify(result)}
      </pre>
    )}
  </div>
);

/** ErrorPart is a turn that produced no answer: the agent's own sentence, unedited. */
function ErrorPart({ data }: { data: { text: string } }) {
  return <p className="agent-panel__error">{data.text}</p>;
}

/** StopPart is the reason a turn ended, for the endings a reader would have to guess at. */
function StopPart({ data }: { data: { reason: string } }) {
  return <p className="agent-panel__stop">the agent stopped: {data.reason}</p>;
}

/** WaitingPart is a message whose turn has answered nothing yet. */
const WaitingPart = () => <p className="agent-panel__waiting">waiting for the agent…</p>;

/** PlanPart is the agent's plan: the steps it published, in its own order. */
function PlanPart({ data }: { data: AgentPlan }) {
  return (
    <ol className="agent-panel__plan">
      {(data.entries ?? []).map((entry) => (
        <li key={entry.content}>
          {entry.content}
          {entry.status ? ` (${entry.status})` : ""}
        </li>
      ))}
    </ol>
  );
}

/**
 * PermissionPart is the approve/deny card as a `data-permission` part: nothing runs
 * unasked, so the agent blocks here until the user answers, and the answer replaces the
 * buttons with the decision the bridge records — the card resolves where it stands.
 */
function PermissionPart({ data }: { data: AgentPermission }) {
  const answer = useContext(AnswerContext);
  return (
    <div className="agent-panel__card">
      <p className="agent-panel__part">
        {data.title ?? "The agent asks for permission"}
        {data.toolKind ? ` (${data.toolKind})` : ""}
      </p>
      {data.decision ? (
        <p className="agent-panel__part agent-panel__decision">{data.decision}</p>
      ) : (
        <div className="agent-panel__card-actions">
          <button
            type="button"
            className="agent-panel__action"
            onClick={() => answer(data.requestId, true)}
          >
            Allow
          </button>
          <button
            type="button"
            className="agent-panel__action"
            onClick={() => answer(data.requestId, false)}
          >
            Deny
          </button>
        </div>
      )}
    </div>
  );
}
