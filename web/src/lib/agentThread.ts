/**
 * The converter between the bridge's thread and assistant-ui's message model.
 *
 * It is written by hand because the state belongs to the bridge, not to the runtime:
 * the Go side accumulates the thread, a `session/update` variant at a time, and this
 * module only says what each part *is* in the UI's terms. Nothing here renames a wire
 * field — the part kinds come from the same constants the client module mirrors — and
 * a part kind this build does not know shows its own name rather than disappearing.
 */
import {
  PART_PERMISSION,
  PART_TEXT,
  PART_THOUGHT,
  PART_TOOL_CALL,
  ROLE_USER,
  type AgentPart,
  type AgentPermission,
  type AgentPlan,
  type AgentThread,
  type AgentToolCall,
} from "./agent";

/**
 * threadMessages maps a whole thread onto assistant-ui's messages.
 *
 * The plan is a thread-level artifact rather than a part of a message — it spans
 * messages and is updated in place by id — so it becomes its own synthetic message
 * under a stable id, which is what lets the runtime update it where it stands.
 */
/**
 * PartLike is one converted part: the shapes this converter produces, stated here rather
 * than squeezed out of the library's union. `threadMessages` casts once at the boundary,
 * and the converter's own tests plus the wire golden keep the cast honest.
 */
export type PartLike =
  | { type: "text"; text: string }
  | { type: "reasoning"; text: string }
  | { type: "data-permission"; data: AgentPermission }
  | { type: "data-plan"; data: AgentPlan }
  | {
      type: "tool-call";
      toolCallId: string;
      toolName: string;
      argsText: string;
      result?: unknown;
      isError: boolean;
      status: "running" | "complete" | "incomplete";
    };

/** MessageStatus is the runtime's own vocabulary for a message: an object union, not a
 *  word — a message waiting on the user is `requires-action`, which is what turns on the
 *  runtime's approval affordances. */
export type MessageStatus =
  | { readonly type: "complete" }
  | { readonly type: "requires-action"; readonly reason: "tool-calls" };

/**
 * ConvertedMessage is one message in the shape this converter produces. The runtime's own
 * message type is a union of normalised forms and experimental parts, and projecting this
 * module's shapes through it produced `never` at every branch — so the shape is stated
 * here and cast once where the messages reach the runtime, which is the boundary that has
 * to agree with the library.
 */
export interface ConvertedMessage {
  id?: string;
  role: "user" | "assistant";
  content: PartLike[];
  status?: MessageStatus;
}

export function messageStatus(parts: AgentPart[]): MessageStatus {
  return parts.some((part) => part.permission && !part.permission.decision)
    ? { type: "requires-action", reason: "tool-calls" }
    : { type: "complete" };
}

export function threadMessages(thread: AgentThread): ConvertedMessage[] {
  const messages: ConvertedMessage[] = thread.messages.map((message) => ({
    id: message.id,
    role: message.role === ROLE_USER ? "user" : "assistant",
    content: message.parts.map(partLike),
    // Only an assistant message carries a status, and only a message waiting on the user
    // is unfinished: the card is part of the turn, and the turn is not done.
    ...(message.role === ROLE_USER ? {} : { status: messageStatus(message.parts) }),
  }));
  if (thread.plan?.entries?.length) {
    messages.push({
      id: "plan",
      role: "assistant",
      content: [{ type: "data-plan", data: thread.plan }],
    });
  }
  return messages;
}

/** partLike maps one part; a kind this build does not know is shown, not dropped. */
export function partLike(part: AgentPart): PartLike {
  switch (part.kind) {
    case PART_TEXT:
      return { type: "text", text: part.text ?? "" };
    case PART_THOUGHT:
      // The agent's reasoning is a reasoning part: collapsed by the runtime, not
      // mixed into what it answered.
      return { type: "reasoning", text: part.text ?? "" };
    case PART_TOOL_CALL:
      return toolCallLike(part.toolCall);
    case PART_PERMISSION:
      // A `data-` part is assistant-ui's own way of carrying something the runtime has
      // no opinion about: the panel renders it through `data.by_name.permission`.
      // A card with no permission at all cannot happen on the wire; the fallback keeps the
      // part renderable rather than dropping it.
      return { type: "data-permission", data: part.permission ?? { requestId: "", sessionId: "" } };
    default:
      return { type: "text", text: part.text ?? part.kind };
  }
}

/** toolCallLike maps a tool call, with the status the bridge reports. */
export function toolCallLike(call?: AgentToolCall): PartLike {
  const toolCall = call ?? { id: "" };
  return {
    type: "tool-call",
    toolCallId: toolCall.id,
    toolName: toolCall.title ?? toolCall.kind ?? "tool",
    argsText: toolCall.input === undefined ? "" : JSON.stringify(toolCall.input),
    result: toolCall.output,
    isError: toolCall.status === "failed",
    // The protocol's status in the runtime's own words: a part that is neither done
    // nor failed is still running.
    status:
      toolCall.status === "completed"
        ? "complete"
        : toolCall.status === "failed"
          ? "incomplete"
          : "running",
  };
}
