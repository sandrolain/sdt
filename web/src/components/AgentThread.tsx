/**
 * AgentThread — the conversation, rendered with assistant-ui.
 *
 * The runtime is `useExternalStoreRuntime`: the bridge owns the state, and
 * `lib/agentThread.ts` is the hand-written converter that says what each part is. The
 * runtime's rendering is taken as vendored, but the parts themselves are this project's:
 * prose goes through the viewer's own markdown path, reasoning is a collapsed block, a
 * tool call is its command and its outcome, and the cards the bridge sends as `data-`
 * parts (the permission card among them) are rendered here with the panel's own tokens.
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
import { createContext, useContext, useMemo, useState } from "react";
import { renderMarkdown, highlightCode } from "../lib/markdown";
import { markdownControlAction } from "../lib/markdownActions";
import { highlightedToolOutput } from "../lib/toolOutput";
import { threadMessages, type ConvertedMessage, type ToolArtifact } from "../lib/agentThread";
import {
  ACTION_ACCEPT,
  ACTION_CANCEL,
  ACTION_DECLINE,
  type AgentElicitation,
  type AgentPermission,
  type AgentPlan,
  type AgentThread as AgentThreadState,
} from "../lib/agent";

/** The answer to a card travels up through a context: a part cannot take a prop. */
const AnswerContext = createContext<(requestId: string, allow: boolean) => void>(() => undefined);
const ElicitationContext = createContext<
  (requestId: string, action: string, content?: Record<string, unknown>) => void
>(() => undefined);

export function AgentThread({
  thread,
  onAnswer,
  onElicitation,
}: {
  thread: AgentThreadState;
  onAnswer: (requestId: string, allow: boolean) => void;
  onElicitation: (requestId: string, action: string, content?: Record<string, unknown>) => void;
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
    // The bridge knows whether a turn is in flight and says so on the thread: the runtime
    // reads the same field the panel does, so the two cannot disagree about whether the
    // agent is working.
    isRunning: Boolean(thread.running),
  });

  return (
    <AnswerContext.Provider value={onAnswer}>
      <ElicitationContext.Provider value={onElicitation}>
        <AssistantRuntimeProvider runtime={runtime}>
          <ThreadPrimitive.Root className="agent-thread">
            <ThreadPrimitive.Viewport className="agent-thread__viewport">
              <ThreadPrimitive.Messages components={{ UserMessage, AssistantMessage }} />
            </ThreadPrimitive.Viewport>
          </ThreadPrimitive.Root>
        </AssistantRuntimeProvider>
      </ElicitationContext.Provider>
    </AnswerContext.Provider>
  );
}

/**
 * The two message shapes are named once, at module scope: a component *type* created inside
 * render is a new type to React on every frame, so every message would remount — taking the
 * open state of anything inside it, and the memoised parse of every prose part, with it.
 */
const UserMessage = () => <MessageRow role="user" />;
const AssistantMessage = () => <MessageRow role="assistant" />;

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
              elicitation: ElicitationPart,
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

/** TextPart is what the agent (or the user) said: prose, rendered as the markdown it is. */
const TextPart: TextMessagePartComponent = ({ text }) => <MarkdownPart markdown={text} />;

/** ReasoningPart is the agent's thinking: collapsed, because it is not the answer. */
const ReasoningPart: ReasoningMessagePartComponent = ({ text }) => (
  <details className="agent-panel__reasoning">
    <summary>Reasoning</summary>
    <MarkdownPart markdown={text} />
  </details>
);

/**
 * MarkdownPart renders prose through the viewer's own markdown path — the corpus renderer
 * that already sanitizes what the agent sends, as it does for a document — and memoises the
 * parse on the part's own text, so a frame that arrived for another part re-renders this one
 * without parsing it again.
 */
function MarkdownPart({ markdown }: { markdown: string }) {
  const html = useMemo(() => renderMarkdown(markdown), [markdown]);
  return (
    <div
      className="agent-panel__markdown"
      // The renderer's code-block controls (wrap, copy) are answered by the same shared
      // action the document page uses; nothing here routes a link, so a click that is not
      // one of those controls keeps the browser's own behaviour.
      onClick={(event) => {
        if (markdownControlAction(event.target)) event.preventDefault();
      }}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}

/**
 * ToolCallPart is one tool the agent ran: the command it was, the outcome it produced, and —
 * behind one disclosure — the protocol's own record of the call. The state is a word as well
 * as a colour, and the envelope is opt-in detail rather than the answer.
 */
const ToolCallPart: ToolCallMessagePartComponent = ({
  toolName,
  argsText,
  result,
  isError,
  artifact,
}) => {
  const raw = artifact as ToolArtifact | undefined;
  // The output is read once: the disclosure's summary states its size, and the body is the
  // text itself, so the reader decides what opening it costs before it takes the thread.
  const output = result === undefined ? undefined : asText(result);
  const highlighted = useMemo(
    () => (output === undefined ? undefined : highlightedToolOutput(output)),
    [output],
  );
  const command = commandOf(raw?.input);
  return (
    <div className={`agent-panel__tool${isError ? " is-error" : ""}`}>
      {/* The state lives in the block's corner and is always a word: colour reinforces it
          and never carries it alone. */}
      <span className="agent-panel__tool-state" data-state={stateKey(raw, isError)}>
        {statusWord(raw, isError)}
      </span>
      <p className="agent-panel__tool-head">
        {command === undefined ? (
          <span className="agent-panel__tool-name">{toolName}</span>
        ) : (
          // The agent's own command line, highlighted as the shell it is. hljs escapes what
          // it is given, so this is the agent's text as text, never as markup.
          <code
            className="agent-panel__tool-command hljs"
            dangerouslySetInnerHTML={{ __html: highlightCode(command, "shell") }}
          />
        )}
      </p>
      {highlighted === undefined ? null : (
        <details className="agent-panel__tool-output">
          <summary>{outputSummary(output ?? "")}</summary>
          <pre className="agent-panel__tool-result">
            <code
              className={`hljs language-${highlighted.language}`}
              dangerouslySetInnerHTML={{ __html: highlighted.html }}
            />
          </pre>
        </details>
      )}
      {argsText || raw?.output !== undefined ? (
        <details className="agent-panel__tool-envelope">
          <summary>Protocol envelope</summary>
          {argsText ? <pre className="agent-panel__tool-args">{argsText}</pre> : null}
          {raw?.output === undefined ? null : (
            <pre className="agent-panel__tool-args">{asText(raw.output)}</pre>
          )}
        </details>
      ) : null}
    </div>
  );
};

/** asText is a value as the panel shows it: a string as it stands, anything else as JSON. */
function asText(value: unknown): string {
  return typeof value === "string" ? value : JSON.stringify(value);
}

/**
 * commandOf is the command line inside a tool's input. Most agents put a shell command in
 * `command`; a call without one keeps the ACP title, which is the only line it has.
 */
function commandOf(input: unknown): string | undefined {
  if (input === null || typeof input !== "object") return undefined;
  const command = (input as { command?: unknown }).command;
  return typeof command === "string" && command.trim() !== "" ? command : undefined;
}

/** outputSummary is what opening the output costs, in the reader's own unit: lines. A
 *  trailing newline ends the last line, it does not start an empty one, so the count is the
 *  number of lines the reader will see. */
function outputSummary(text: string): string {
  const body = text.endsWith("\n") ? text.slice(0, -1) : text;
  const lines = body.split("\n").length;
  return `Output · ${lines} ${lines === 1 ? "line" : "lines"}`;
}

/** stateKey is the call's state as the stylesheet's own handle on it. */
function stateKey(raw: ToolArtifact | undefined, isError: boolean | undefined): string {
  if (isError || raw?.status === "failed") return "failed";
  return raw?.status === "completed" ? "completed" : "running";
}

/** statusWord is the call's state in a word, so it never rests on the colour alone. It is
 *  read from the agent's own status: the runtime gives a part with no result the message's
 *  status, which in this panel is always complete, so a call in flight would read finished.
 *  A word the panel does not know is treated as unfinished rather than as done. */
function statusWord(raw: ToolArtifact | undefined, isError: boolean | undefined): string {
  if (isError || raw?.status === "failed") return "Failed";
  return raw?.status === "completed" ? "Completed" : "Running";
}

/**
 * ElicitationPart is the agent's question. The card renders the message and one control
 * per requested property, and answers through the same command path the permission card
 * uses; once answered it keeps the decision and the values in place, so the thread is the
 * record of what was asked and what was chosen.
 *
 * The schema is the protocol's own: this build renders the four primitive shapes it can
 * (string, number, boolean, enum) and declines a question whose fields it cannot draw
 * rather than sending the agent a half-filled form.
 */
const ELICITATION_LABELS: Record<string, string> = {
  accepted: "accepted",
  declined: "declined",
  cancelled: "cancelled",
};

interface ElicitationField {
  name: string;
  kind: "string" | "number" | "boolean" | "enum" | "unknown";
  title: string;
  required: boolean;
  choices?: string[];
}

function elicitationFields(request: AgentElicitation): ElicitationField[] {
  const properties = request.schema?.properties ?? {};
  const required = new Set(request.schema?.required ?? []);
  return Object.entries(properties).map(([name, value]) => {
    const spec = (value ?? {}) as Record<string, unknown>;
    const choices = Array.isArray(spec.enum)
      ? spec.enum.map((choice) => String(choice))
      : undefined;
    const kind: ElicitationField["kind"] = choices
      ? "enum"
      : spec.type === "number" || spec.type === "integer"
        ? "number"
        : spec.type === "boolean"
          ? "boolean"
          : spec.type === "string"
            ? "string"
            : "unknown";
    return {
      name,
      kind,
      title: typeof spec.title === "string" ? spec.title : name,
      required: required.has(name),
      choices,
    };
  });
}

/** initialValues is the draft a reader starts from: a boolean is false, everything else
 *  empty, so an untouched required field is visibly missing. */
function initialValues(fields: ElicitationField[]): Record<string, string | boolean> {
  const values: Record<string, string | boolean> = {};
  for (const field of fields) values[field.name] = field.kind === "boolean" ? false : "";
  return values;
}

/** elicitedContent turns the draft into the values the agent asked for: numbers as
 *  numbers, booleans as booleans, everything else as typed. */
function elicitedContent(
  fields: ElicitationField[],
  values: Record<string, string | boolean>,
): Record<string, unknown> {
  const content: Record<string, unknown> = {};
  for (const field of fields) {
    const value = values[field.name];
    if (field.kind === "unknown") continue;
    if (field.kind === "boolean") content[field.name] = Boolean(value);
    else if (field.kind === "number") content[field.name] = value === "" ? 0 : Number(value);
    else content[field.name] = String(value ?? "");
  }
  return content;
}

function ElicitationPart({ data }: { data: AgentElicitation }) {
  const answer = useContext(ElicitationContext);
  const fields = useMemo(() => elicitationFields(data), [data]);
  const [values, setValues] = useState(() => initialValues(fields));
  const unsupported = fields.some((field) => field.kind === "unknown");
  const missing = fields.some(
    (field) => field.required && !data.decision && isBlank(values[field.name]),
  );

  return (
    <div className="agent-panel__card agent-panel__elicitation">
      <p className="agent-panel__part">
        {data.message === "" ? "The agent asks a question" : data.message}
      </p>

      {data.decision ? (
        <>
          <p className="agent-panel__part agent-panel__decision">
            {ELICITATION_LABELS[data.decision] ?? data.decision}
          </p>
          {data.content && Object.keys(data.content).length > 0 ? (
            <dl className="agent-panel__elicitation-values">
              {Object.entries(data.content).map(([name, value]) => (
                <div key={name}>
                  <dt>{name}</dt>
                  <dd>{String(value)}</dd>
                </div>
              ))}
            </dl>
          ) : null}
        </>
      ) : data.mode === "url" ? (
        <>
          <p className="agent-panel__part">
            <a href={data.url} target="_blank" rel="noopener noreferrer">
              Open {data.url}
            </a>
          </p>
          <p className="agent-panel__note">
            {data.completed ? "Completed." : "Finish there; the agent continues when it is done."}
          </p>
          <div className="agent-panel__card-actions">
            <button
              type="button"
              className="agent-panel__action agent-panel__action--secondary"
              onClick={() => answer(data.requestId, ACTION_CANCEL)}
            >
              Cancel
            </button>
          </div>
        </>
      ) : (
        <>
          {unsupported ? (
            <p className="agent-panel__note">
              This question asks for a field this panel cannot render. Decline it and ask the agent
              another way.
            </p>
          ) : null}
          {fields.length > 0 ? (
            <div className="agent-panel__elicitation-fields">
              {fields.map((field) => (
                <label key={field.name} className="agent-panel__field">
                  <span>
                    {field.title}
                    {field.required ? " *" : ""}
                  </span>
                  {field.kind === "boolean" ? (
                    <input
                      type="checkbox"
                      checked={Boolean(values[field.name])}
                      onChange={(event) =>
                        setValues((current) => ({ ...current, [field.name]: event.target.checked }))
                      }
                    />
                  ) : field.kind === "enum" ? (
                    <select
                      aria-required={field.required}
                      value={String(values[field.name] ?? "")}
                      onChange={(event) =>
                        setValues((current) => ({ ...current, [field.name]: event.target.value }))
                      }
                    >
                      <option value="">Choose…</option>
                      {(field.choices ?? []).map((choice) => (
                        <option key={choice} value={choice}>
                          {choice}
                        </option>
                      ))}
                    </select>
                  ) : (
                    <input
                      type={field.kind === "number" ? "number" : "text"}
                      aria-required={field.required}
                      disabled={field.kind === "unknown"}
                      value={String(values[field.name] ?? "")}
                      onChange={(event) =>
                        setValues((current) => ({ ...current, [field.name]: event.target.value }))
                      }
                    />
                  )}
                </label>
              ))}
            </div>
          ) : null}
          <div className="agent-panel__card-actions">
            <button
              type="button"
              className="agent-panel__action"
              disabled={unsupported || missing}
              onClick={() => answer(data.requestId, ACTION_ACCEPT, elicitedContent(fields, values))}
            >
              Accept
            </button>
            <button
              type="button"
              className="agent-panel__action agent-panel__action--secondary"
              onClick={() => answer(data.requestId, ACTION_DECLINE)}
            >
              Decline
            </button>
          </div>
        </>
      )}
    </div>
  );
}

function isBlank(value: string | boolean | undefined): boolean {
  return value === undefined || value === "" || value === false;
}

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
            className="agent-panel__action agent-panel__action--secondary"
            onClick={() => answer(data.requestId, false)}
          >
            Deny
          </button>
        </div>
      )}
    </div>
  );
}
