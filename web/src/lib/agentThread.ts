/**
 * The converter between the bridge's thread and assistant-ui's message model.
 *
 * It is written by hand because the state belongs to the bridge, not to the runtime:
 * the Go side accumulates the thread, a `session/update` variant at a time, and this
 * module only says what each part *is* in the UI's terms. Nothing here renames a wire
 * field — the part kinds come from the same constants the client module mirrors — and
 * a part kind this build does not know shows its own name rather than disappearing.
 */
import type { ThreadMessageLike } from "@assistant-ui/react";
import {
  PART_PERMISSION,
  PART_TEXT,
  PART_THOUGHT,
  PART_TOOL_CALL,
  ROLE_USER,
  type AgentPart,
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
export function threadMessages(thread: AgentThread): ThreadMessageLike[] {
  const messages: ThreadMessageLike[] = thread.messages.map((message) => ({
    id: message.id,
    role: message.role === ROLE_USER ? "user" : "assistant",
    content: message.parts.map(partLike),
    // A message waiting on the user is exactly what assistant-ui calls
    // `requires-action`: the card is part of the turn, and the turn is not done. Only
    // an assistant message carries a status, so a user's does not.
    ...(message.role === ROLE_USER
      ? {}
      : {
          status: message.parts.some((part) => part.permission && !part.permission.decision)
            ? ("requires-action" as const)
            : ("complete" as const),
        }),
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
export function partLike(
  part: AgentPart,
): ThreadMessageLike["content"] extends readonly (infer P)[] ? P : never {
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
      return { type: "data-permission", data: part.permission };
    default:
      return { type: "text", text: part.text ?? part.kind };
  }
}

/** toolCallLike maps a tool call, with the status the bridge reports. */
export function toolCallLike(
  call?: AgentToolCall,
): ThreadMessageLike["content"] extends readonly (infer P)[] ? P : never {
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
