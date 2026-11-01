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
  ActionBarPrimitive,
  AuiIf,
  AssistantRuntimeProvider,
  MessageNotSentError,
  MessagePrimitive,
  ThreadPrimitive,
  groupPartByType,
  useAui,
  useAuiState,
  useExternalStoreRuntime,
} from "@assistant-ui/react";
import type {
  ThreadMessageLike,
  ToolCallMessagePartComponent,
  TextMessagePartComponent,
  ReasoningMessagePartComponent,
} from "@assistant-ui/react";
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ComponentProps,
  type ReactNode,
} from "react";
import { Icon } from "../lib/icon";
import { TooltipButton } from "./ui/Tooltip";
import { renderMarkdown, highlightCode } from "../lib/markdown";
import { Select } from "./ui/Select";
import { MultiSelect } from "./ui/MultiSelect";
import { renderMermaid } from "../lib/mermaidRender";
import { markdownControlAction } from "../lib/markdownActions";
import { highlightedToolOutput } from "../lib/toolOutput";
import { threadMessages, type ConvertedMessage, type ToolArtifact } from "../lib/agentThread";
import {
  ACTION_ACCEPT,
  ACTION_CANCEL,
  ACTION_DECLINE,
  type AgentElicitation,
  type AgentPermission,
  type AgentPermissionOption,
  type AgentPlan,
  type AgentThread as AgentThreadState,
} from "../lib/agent";

/** The answer to a card travels up through a context: a part cannot take a prop. The
 *  answer is the id of the option the reader chose. */
const AnswerContext = createContext<(requestId: string, optionId: string) => void>(() => undefined);
const ElicitationContext = createContext<
  (requestId: string, action: string, content?: Record<string, unknown>) => void
>(() => undefined);

export function AgentThread({
  thread,
  onAnswer,
  onElicitation,
  onSend,
  onCancel,
  children,
}: {
  thread: AgentThreadState;
  onAnswer: (requestId: string, optionId: string) => void;
  onElicitation: (requestId: string, action: string, content?: Record<string, unknown>) => void;
  /** Sends one prompt through the bridge; a rejection is the refusal the reader must see. */
  onSend: (text: string) => Promise<void>;
  /** Stops the turn in flight. */
  onCancel: () => void;
  /** The panel's own footer (the composer among it), which has to live inside the runtime. */
  children?: ReactNode;
}) {
  // The converter is where the runtime expects one: `messages` are the bridge's own
  // shape and the runtime normalises them (metadata included) on the way into its store.
  const runtime = useExternalStoreRuntime<ConvertedMessage>({
    messages: useMemo(() => threadMessages(thread), [thread]),
    // The one cast the library's union forces: the converter states its own part shapes and
    // the runtime normalises them on the way into its store.
    convertMessage: (message) => message as unknown as ThreadMessageLike,
    // The runtime owns send and stop, so the panel has one send path rather than two: the
    // text is what the reader typed, and a refusal is handed back as `MessageNotSentError`
    // — the runtime's own way of restoring the draft it cleared at dispatch.
    onNew: async (message) => {
      const text = message.content
        .map((part) => (part.type === "text" ? part.text : ""))
        .filter((chunk) => chunk.trim() !== "")
        .join("\n\n");
      if (text === "") return;
      try {
        await onSend(text);
      } catch (error) {
        throw new MessageNotSentError((error as Error).message);
      }
    },
    onCancel: async () => onCancel(),
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
              {/* Empty is the converter's emptiness: a turn in flight carries its own
                  waiting message, so this never appears over a starting turn. */}
              <AuiIf condition={(state) => state.thread.messages.length === 0}>
                <EmptyThread />
              </AuiIf>
              <ThreadPrimitive.Messages components={{ UserMessage, AssistantMessage }} />
            </ThreadPrimitive.Viewport>
            {/* The runtime knows whether the viewport is at the bottom, so the chip's
                disabled state is its own answer rather than a scroll listener here. */}
            <ThreadPrimitive.ScrollToBottom className="agent-thread__to-bottom">
              Jump to the latest message
            </ThreadPrimitive.ScrollToBottom>
          </ThreadPrimitive.Root>
          {children}
        </AssistantRuntimeProvider>
      </ElicitationContext.Provider>
    </AnswerContext.Provider>
  );
}

/**
 * STARTER_PROMPTS is what an empty thread offers: three sentences about this corpus, each
 * one a question the agent can answer by reading it. They are fixed strings rather than
 * generated suggestions, because the bridge has no suggestion source to generate from.
 */
const STARTER_PROMPTS = [
  "Summarise the plan that is in progress.",
  "Search the corpus for the decisions about this area.",
  "Where does this document live, and what links to it?",
];

/**
 * EmptyThread is the thread with nothing in it: one sentence that names the surface and
 * three starters. A starter goes through the runtime's own send path, so the empty state
 * and the composer can never take two different roads to the bridge.
 */
function EmptyThread() {
  const aui = useAui();
  return (
    <div className="agent-thread__empty">
      <p className="agent-thread__empty-lede">
        The agent this project declares, over the corpus you are reading.
      </p>
      <div className="agent-thread__starters">
        {STARTER_PROMPTS.map((prompt) => (
          <button
            key={prompt}
            type="button"
            className="agent-thread__starter"
            onClick={() => {
              aui.composer.setText(prompt);
              aui.composer.send();
            }}
          >
            {prompt}
          </button>
        ))}
      </div>
    </div>
  );
}

/**
 * The two message shapes are named once, at module scope: a component *type* created inside
 * render is a new type to React on every frame, so every message would remount — taking the
 * open state of anything inside it, and the memoised parse of every prose part, with it.
 */
const UserMessage = () => <MessageRow role="user" />;
const AssistantMessage = () => <MessageRow role="assistant" />;

/**
 * CHAT_GROUPS is how a turn's parts fold: reasoning and tool calls share one chain
 * (`group-chain`), and the tools inside it keep their own group (`group-tool`) so a run of
 * commands reads as one row rather than as one block per call. `groupPartByType` carries a
 * stable fingerprint, so the tree survives the frames that arrive for another part.
 */
const CHAT_GROUPS = groupPartByType({
  reasoning: ["group-chain"],
  "tool-call": ["group-chain", "group-tool"],
});

/** RenderInfo is the library's own render signature, derived rather than restated. */
type RenderInfo = Parameters<
  NonNullable<ComponentProps<typeof MessagePrimitive.GroupedParts>["children"]>
>[0];

/** GroupLike is what a group node carries that this panel reads. */
type GroupLike = {
  readonly indices: readonly number[];
  readonly status: { readonly type: string };
};

/** MessageRow renders one message; the runtime fills the parts. */
function MessageRow({ role }: { role: "user" | "assistant" }) {
  return (
    <MessagePrimitive.Root className={`agent-panel__message agent-panel__message--${role}`}>
      <MessagePrimitive.GroupedParts groupBy={CHAT_GROUPS}>
        {renderPart}
      </MessagePrimitive.GroupedParts>
      {role === "assistant" ? <MessageActions /> : null}
    </MessagePrimitive.Root>
  );
}

/**
 * renderPart is the one switch over what a part is: a group wraps its children, a leaf is
 * the renderer this panel already had, and a data part is the card its name asks for. It
 * lives at module scope because a function created during render would be a new value on
 * every frame.
 */
function renderPart({ part, children }: RenderInfo) {
  switch (part.type) {
    case "group-chain":
      return <ChainGroup part={part}>{children}</ChainGroup>;
    case "group-tool":
      return <div className="agent-panel__tool-group">{children}</div>;
    case "text":
      return <TextPart {...part} />;
    case "reasoning":
      return <ReasoningPart {...part} />;
    case "tool-call":
      return <ToolCallPart {...part} />;
    case "data":
      return <DataPart part={part} />;
    default:
      return null;
  }
}

/**
 * ChainGroup is one turn's work behind a single line that says how much of it there is.
 * It opens while the turn is in flight and collapses when it ends, and a click is the
 * reader's own answer for as long as the turn lasts: the state is reset during render when
 * the run starts or ends, React's own pattern for a value derived from a prop.
 */
function ChainGroup({ part, children }: { part: GroupLike; children: ReactNode }) {
  // The turn is the reader's unit: a chain is open while the turn runs, even when every part
  // inside it has already finished — the agent is still working, and the work is what is
  // happening. The group's own status is the fallback for a transcript rendered without a run.
  const turnRunning = useAuiState((state) => state.thread.isRunning);
  const running = turnRunning || part.status.type === "running";
  const [open, setOpen] = useState(running);
  const [wasRunning, setWasRunning] = useState(running);
  if (wasRunning !== running) {
    setWasRunning(running);
    setOpen(running);
  }
  const steps = part.indices.length;
  return (
    <div className="agent-panel__chain">
      <button
        type="button"
        className="agent-panel__chain-head"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
      >
        <span className="agent-panel__chain-count">
          Worked through {steps} {steps === 1 ? "step" : "steps"}
        </span>
        <span className="agent-panel__chain-state" data-state={running ? "running" : "completed"}>
          {running ? "Running" : "Completed"}
        </span>
      </button>
      {open ? <div className="agent-panel__chain-body">{children}</div> : null}
    </div>
  );
}

/** DataPart is the card a `data-` part asks for, by the name the bridge gave it. */
function DataPart({ part }: { part: { readonly name: string; readonly data: unknown } }) {
  switch (part.name) {
    case "permission":
      return <PermissionPart data={part.data as AgentPermission} />;
    case "elicitation":
      return <ElicitationPart data={part.data as AgentElicitation} />;
    case "plan":
      return <PlanPart data={part.data as AgentPlan} />;
    case "error":
      return <ErrorPart data={part.data as { text: string }} />;
    case "stop":
      return <StopPart data={part.data as { reason: string }} />;
    case "waiting":
      return <WaitingPart />;
    default:
      return null;
  }
}

/**
 * MessageActions is what a reader can do with an answer: copy its text, or copy the same
 * thing as the markdown the agent wrote. The text copy is the runtime's own (it knows what
 * a message's copyable content is); the markdown copy is the message's text parts joined,
 * which is the source the renderer was given — not a re-serialisation of the HTML.
 */
function MessageActions() {
  const parts = useAuiState((state) => state.message.parts);
  // The copied state is a word: the runtime says when the message is on the clipboard, and the
  // control repeats it rather than changing an icon the reader has to interpret.
  const copied = useAuiState((state) => state.message.isCopied);
  const [markdownCopied, setMarkdownCopied] = useState(false);
  const markdown = useMemo(
    () =>
      parts
        .map((part) => (part.type === "text" ? part.text : ""))
        .filter((chunk) => chunk.trim() !== "")
        .join("\n\n"),
    [parts],
  );
  return (
    <ActionBarPrimitive.Root className="agent-panel__actions">
      <ActionBarPrimitive.Copy
        render={
          <TooltipButton
            label={copied ? "Copied" : "Copy"}
            tooltip={copied ? "Copied" : "Copy this answer"}
            className="agent-panel__icon"
          />
        }
      >
        <Icon name="content_copy" />
      </ActionBarPrimitive.Copy>
      <TooltipButton
        label={markdownCopied ? "Copied as markdown" : "Copy as markdown"}
        tooltip={markdownCopied ? "Copied as markdown" : "Copy this answer as markdown"}
        className="agent-panel__icon"
        onPress={() => {
          void navigator.clipboard.writeText(markdown);
          setMarkdownCopied(true);
        }}
      >
        <Icon name="code" />
      </TooltipButton>
    </ActionBarPrimitive.Root>
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
  // `breaks` is on for the chat and off for a document: here the newline is the reader's
  // own, and prose typed or written in lines must keep them.
  const html = useMemo(() => renderMarkdown(markdown, { breaks: true }), [markdown]);
  const host = useRef<HTMLDivElement | null>(null);
  // The document page's own mount: a mermaid fence becomes a placeholder here, and the
  // renderer is what turns it into a diagram. It returns before importing mermaid when the
  // part carries none, so an answer without a diagram pays nothing for this effect.
  useEffect(() => {
    void renderMermaid(host.current);
  }, [html]);
  return (
    <div
      ref={host}
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
  kind: "string" | "number" | "boolean" | "enum" | "multi-enum" | "unknown";
  title: string;
  required: boolean;
  choices?: string[];
}

/** ElicitationValue is what a field's draft holds: a boolean, an array of choices, or text. */
type ElicitationValue = string | boolean | string[];

function elicitationFields(request: AgentElicitation): ElicitationField[] {
  const properties = request.schema?.properties ?? {};
  const required = new Set(request.schema?.required ?? []);
  return Object.entries(properties).map(([name, value]) => {
    const spec = (value ?? {}) as Record<string, unknown>;
    // An array of enums is a set of answers, not an unknown: the choices are the items it
    // names, and the card renders one control per choice.
    const items = (spec.items ?? {}) as Record<string, unknown>;
    const choices = Array.isArray(spec.enum)
      ? spec.enum.map((choice) => String(choice))
      : spec.type === "array" && Array.isArray(items.enum)
        ? items.enum.map((choice) => String(choice))
        : undefined;
    const kind: ElicitationField["kind"] = choices
      ? spec.type === "array"
        ? "multi-enum"
        : "enum"
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
function initialValues(fields: ElicitationField[]): Record<string, ElicitationValue> {
  const values: Record<string, ElicitationValue> = {};
  for (const field of fields) {
    values[field.name] = field.kind === "boolean" ? false : field.kind === "multi-enum" ? [] : "";
  }
  return values;
}

/** elicitedContent turns the draft into the values the agent asked for: numbers as
 *  numbers, booleans as booleans, everything else as typed. */
function elicitedContent(
  fields: ElicitationField[],
  values: Record<string, ElicitationValue>,
): Record<string, unknown> {
  const content: Record<string, unknown> = {};
  for (const field of fields) {
    const value = values[field.name];
    if (field.kind === "unknown") continue;
    if (field.kind === "boolean") content[field.name] = Boolean(value);
    else if (field.kind === "multi-enum") content[field.name] = Array.isArray(value) ? value : [];
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
              {fields.map((field) => {
                const title = (
                  <span>
                    {field.title}
                    {field.required ? " *" : ""}
                  </span>
                );
                // A choice is the app's own control, and a button-based control is not a
                // form field a <label> can wrap: the control carries its own label, which is
                // also what names the trigger for a reader who cannot see it.
                if (field.kind === "multi-enum") {
                  return (
                    <MultiSelect
                      key={field.name}
                      label={`${field.title}${field.required ? " *" : ""}`}
                      ariaLabel={field.title}
                      options={(field.choices ?? []).map((choice) => ({
                        id: choice,
                        label: choice,
                      }))}
                      selected={
                        Array.isArray(values[field.name]) ? (values[field.name] as string[]) : []
                      }
                      onChange={(ids) =>
                        setValues((current) => ({ ...current, [field.name]: ids }))
                      }
                    />
                  );
                }
                if (field.kind === "enum") {
                  return (
                    <Select
                      key={field.name}
                      label={`${field.title}${field.required ? " *" : ""}`}
                      ariaLabel={field.title}
                      options={[
                        { id: "", label: "Choose…" },
                        ...(field.choices ?? []).map((choice) => ({
                          id: choice,
                          label: choice,
                        })),
                      ]}
                      selectedKey={String(values[field.name] ?? "")}
                      onSelectionChange={(key) =>
                        setValues((current) => ({ ...current, [field.name]: String(key) }))
                      }
                    />
                  );
                }
                return (
                  <label key={field.name} className="agent-panel__field">
                    {title}
                    {field.kind === "boolean" ? (
                      <input
                        type="checkbox"
                        checked={Boolean(values[field.name])}
                        onChange={(event) =>
                          setValues((current) => ({
                            ...current,
                            [field.name]: event.target.checked,
                          }))
                        }
                      />
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
                );
              })}
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

/** isBlank is a field with nothing in it. A boolean is never blank: `false` is an answer,
 *  and a required checkbox must not be able to block the form it belongs to. */
function isBlank(value: ElicitationValue | undefined): boolean {
  if (Array.isArray(value)) return value.length === 0;
  return value === undefined || value === "";
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
 * PermissionPart is the card the agent asked with: one control per option the agent
 * offered, labelled in the agent's own words, so a request that offers several answers is
 * a choice rather than a yes/no. Nothing runs unasked, so the agent blocks here until the
 * reader answers, and the answer replaces the buttons with the choice the bridge records.
 */
function PermissionPart({ data }: { data: AgentPermission }) {
  const answer = useContext(AnswerContext);
  const options = data.options ?? [];
  // The recorded decision is an option id; the name the reader met comes from the same
  // request, so the card is the record of what was asked and what was chosen.
  const chosen = options.find((option) => option.optionId === data.decision);
  return (
    <div className="agent-panel__card">
      <p className="agent-panel__part">
        {data.title ?? "The agent asks for permission"}
        {data.toolKind ? ` (${data.toolKind})` : ""}
      </p>
      {data.decision ? (
        <p className="agent-panel__part agent-panel__decision">{chosen?.name ?? data.decision}</p>
      ) : options.length === 0 ? (
        <p className="agent-panel__note">
          This request names no option this panel can show. Stop the turn and ask the agent another
          way.
        </p>
      ) : (
        <div className="agent-panel__card-actions">
          {options.map((option) => (
            <button
              key={option.optionId}
              type="button"
              className={`agent-panel__action${
                isRefusal(option) ? " agent-panel__action--secondary" : ""
              }`}
              onClick={() => answer(data.requestId, option.optionId)}
            >
              {option.name}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/** isRefusal says whether an option refuses, which the card renders as the secondary
 *  control. The kind is the protocol's hint, and a question's options are all allow_*: the
 *  hierarchy never decides the answer. */
function isRefusal(option: AgentPermissionOption): boolean {
  return option.kind === "reject_once" || option.kind === "reject_always";
}
