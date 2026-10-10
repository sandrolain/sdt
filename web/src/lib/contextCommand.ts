/**
 * Document-state actions, delegated to the sanctioned CLI mutator (decision
 * 0027). The viewer never writes document bytes: it asks the server, which
 * execs `sdt context close` / `sdt context status set` — the same commands and
 * the same guardrails the agent uses.
 */

/** Whether an action can run, and why not when it cannot. */
export interface CommandCapability {
  available: boolean;
  reason?: string;
}

/** A document-state request; `action` is a closed set on the server. */
export interface CommandRequest {
  doc: string;
  action: "close" | "status-set";
  value?: string;
  reason?: string;
}

/** Kinds the CLI can close; the viewer hides the action for anything else. */
export const CLOSABLE_KINDS = new Set(["tasks", "plan", "analysis"]);

const COMMAND_PATH = "/api/context/command";

/**
 * Ask whether an action can run. A viewer serving a corpus without the CLI, or
 * bound to a non-loopback host, answers `{available:false, reason}` — the
 * caller disables its control and shows that reason instead of failing on click.
 */
export async function loadCommandCapability(): Promise<CommandCapability> {
  try {
    const res = await fetch(COMMAND_PATH);
    if (!res.ok) return { available: false, reason: `HTTP ${res.status}` };
    return (await res.json()) as CommandCapability;
  } catch {
    return { available: false, reason: "the viewer server is unreachable" };
  }
}

/** Run one action. The raised message is the server's refusal, verbatim. */
export async function runCommand(req: CommandRequest): Promise<void> {
  const res = await fetch(COMMAND_PATH, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new Error(body?.error ?? `HTTP ${res.status}`);
  }
}
