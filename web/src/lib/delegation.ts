import { fetchJSON } from "./fetchJson";

export type DelegatedTask = {
  id: string;
  status: string;
  role: string;
  agent?: string;
  model?: string;
  created_at: string;
  latest_attempt_started_at?: string;
  plan?: string;
  phase?: string;
  workspace?: string;
  deadline?: string;
  pending_permissions?: string[];
  pending_permission_count?: number;
  trace_artifacts?: string[];
  recent_activity?: { id: number; at: string; type: string }[];
  result_ready: boolean;
  next_action: string;
};

export type DelegatedTasksSnapshot = { tasks: DelegatedTask[]; omitted?: number };

/** A linked task without a phase owns the entire plan. */
export function planScopeLabel(phase?: string): string {
  return phase?.trim() || "Entire plan";
}

export async function fetchDelegatedTasks(): Promise<DelegatedTasksSnapshot> {
  return fetchJSON<DelegatedTasksSnapshot>("/api/delegation/tasks");
}
