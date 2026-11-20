import { useEffect, useRef, useState, type ReactNode, type SyntheticEvent } from "react";
import { Link } from "react-router-dom";
import { formatFieldDateTime } from "../lib/frontmatter";
import { Icon } from "../lib/icon";
import {
  fetchDelegatedTasks,
  planScopeLabel,
  type DelegatedTask,
  type DelegatedTasksSnapshot,
} from "../lib/delegation";

const refreshIntervalMS = 5000;

export function DelegationPage() {
  const [snapshot, setSnapshot] = useState<DelegatedTasksSnapshot | null>(null);
  const [selectedTaskID, setSelectedTaskID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [refresh, setRefresh] = useState(0);
  const loading = useRef(false);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      if (loading.current) return;
      loading.current = true;
      fetchDelegatedTasks()
        .then((value) => {
          if (cancelled) return;
          setSnapshot(value);
          setError(null);
        })
        .catch((reason: unknown) => {
          if (!cancelled) setError(reason instanceof Error ? reason.message : String(reason));
        })
        .finally(() => {
          loading.current = false;
        });
    };
    load();
    const timer = window.setInterval(load, refreshIntervalMS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [refresh]);

  const selectedTask =
    snapshot?.tasks.find((task) => task.id === selectedTaskID) ?? snapshot?.tasks[0] ?? null;

  return (
    <main className="content">
      <header className="delegation-header">
        <div>
          <p className="delegation-eyebrow">Principal workspace</p>
          <h1>Delegated tasks</h1>
          <p className="oc-muted">Monitor work assigned through the SDT CLI.</p>
        </div>
        <div className="delegation-header__actions">
          <span className="delegation-refresh-state" role="status">
            Auto-refresh · 5 sec
          </span>
          <button
            type="button"
            className="search-trigger"
            aria-label="Refresh delegated tasks now"
            onClick={() => setRefresh((value) => value + 1)}
          >
            Refresh now
          </button>
        </div>
      </header>

      {error ? (
        <p className="delegation-alert" role="alert">
          Task status unavailable: {error}
        </p>
      ) : null}
      {snapshot?.omitted ? (
        <p className="delegation-notice" role="status">
          Showing the 100 most recent tasks; {snapshot.omitted} older tasks omitted.
        </p>
      ) : null}
      {snapshot === null && !error ? (
        <p className="content__empty">Loading delegated tasks…</p>
      ) : null}
      {snapshot ? (
        <section
          className="delegation-list delegation-inbox"
          aria-labelledby="delegation-inbox-title"
        >
          <div className="delegation-list__header">
            <h2 id="delegation-inbox-title">Principal inbox</h2>
            <span>{snapshot.inbox?.length ?? 0}</span>
          </div>
          {snapshot.inbox?.length ? (
            <ul>
              {snapshot.inbox.map((item) => {
                const task = snapshot.tasks.find((candidate) => candidate.id === item.task_id);
                return (
                  <li key={`${item.task_id}:${item.revision}`}>
                    <button
                      type="button"
                      className="delegation-task"
                      aria-pressed={selectedTask?.id === item.task_id}
                      onClick={() => setSelectedTaskID(item.task_id)}
                    >
                      <span className="delegation-task__heading">
                        <span className="delegation-task__role">{task?.role ?? item.task_id}</span>
                        <span
                          className={`delegation-task__status delegation-task__status--${statusTone(item.status)}`}
                        >
                          <Icon name={statusIcon(item.status)} />
                          {item.kinds.map(statusLabel).join(" · ")}
                        </span>
                      </span>
                      <span className="delegation-task__meta">
                        {item.pending_request_ids?.length ? (
                          <>
                            <span>
                              {item.pending_request_ids.length} pending permission request
                              {item.pending_request_ids.length === 1 ? "" : "s"}
                            </span>
                            <span>Requests: {item.pending_request_ids.join(", ")}</span>
                          </>
                        ) : null}
                        <span>{item.next_action} · CLI response</span>
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          ) : (
            <p className="delegation-inbox__empty">No task decisions are pending.</p>
          )}
        </section>
      ) : null}
      {snapshot?.tasks.length === 0 ? (
        <p className="content__empty">No delegated tasks are recorded for this project.</p>
      ) : null}

      {snapshot && snapshot.tasks.length > 0 ? (
        <section className="delegation-workspace" aria-label="Delegated task workspace">
          <section className="delegation-list" aria-label="Tasks">
            <div className="delegation-list__header">
              <h2>Tasks</h2>
              <span>{snapshot.tasks.length}</span>
            </div>
            <ul>
              {snapshot.tasks.map((task) => (
                <li key={task.id}>
                  <button
                    type="button"
                    className="delegation-task"
                    aria-pressed={selectedTask?.id === task.id}
                    onClick={() => setSelectedTaskID(task.id)}
                  >
                    <span className="delegation-task__heading">
                      <span className="delegation-task__role">{task.role}</span>
                      <span
                        className={`delegation-task__status delegation-task__status--${statusTone(task.status)}`}
                      >
                        <Icon name={statusIcon(task.status)} />
                        {statusLabel(task.status)}
                      </span>
                    </span>
                    <span className="delegation-task__meta">
                      <span
                        className="delegation-task__agent"
                        title={agentLabel(task) || "Waiting for agent"}
                      >
                        <Icon name="smart_toy" />
                        <span>{agentLabel(task) || "Waiting for agent"}</span>
                      </span>
                      <time
                        className="delegation-task__requested"
                        dateTime={task.created_at}
                        title="Requested"
                      >
                        <Icon name="schedule" />
                        <span>{formatFieldDateTime(task.created_at)}</span>
                      </time>
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </section>
          {selectedTask ? <TaskDetails task={selectedTask} /> : null}
        </section>
      ) : null}
    </main>
  );
}

function TaskDetails({ task }: { task: DelegatedTask }) {
  return (
    <article className="delegation-detail" aria-labelledby="delegation-detail-title">
      <header className="delegation-detail__header">
        <div className="delegation-detail__status">
          <Icon
            name={statusIcon(task.status)}
            className={`delegation-detail__status-icon delegation-detail__status-icon--${statusTone(task.status)}`}
          />
          <div>
            <p className="delegation-eyebrow">{task.role}</p>
            <h2 id="delegation-detail-title">{statusLabel(task.status)}</h2>
          </div>
        </div>
        {task.result_ready ? (
          <span className="delegation-result-state">
            <Icon name="task_alt" />
            Result ready
          </span>
        ) : null}
      </header>

      <p className="delegation-detail__id">
        <code>{task.id}</code>
      </p>

      <dl className="delegation-metadata">
        <Metadata
          label="Requested"
          value={<time dateTime={task.created_at}>{formatFieldDateTime(task.created_at)}</time>}
        />
        <Metadata
          label="Latest attempt started"
          value={
            task.latest_attempt_started_at ? (
              <time dateTime={task.latest_attempt_started_at}>
                {formatFieldDateTime(task.latest_attempt_started_at)}
              </time>
            ) : (
              "Not started"
            )
          }
        />
        <Metadata label="Agent" value={task.agent || "Not assigned"} />
        <Metadata label="Model" value={task.model || "Unknown"} />
        <Metadata label="Workspace" value={task.workspace || "Not created"} mono />
        <Metadata
          label="Plan scope"
          value={
            task.plan ? (
              <>
                <Link to={`/docs/${task.plan}`}>{task.plan}</Link>
                {` · ${planScopeLabel(task.phase)}`}
              </>
            ) : (
              "No plan linked"
            )
          }
        />
        {task.deadline ? <Metadata label="Deadline" value={task.deadline} mono /> : null}
        {task.pending_permission_count ? (
          <Metadata
            label="Pending permissions"
            value={`${task.pending_permission_count}${task.pending_permissions?.length ? ` · ${task.pending_permissions.join(", ")}` : ""}`}
          />
        ) : null}
      </dl>

      {task.verification ? (
        <section className="delegation-next" aria-label="Declared checks">
          <span>Declared checks · {task.verification.status}</span>
          <ul>
            {task.verification.checks?.map((check) => (
              <li key={check.name}>
                {check.name} · {check.status}
                {check.exit_code ? ` · exit ${check.exit_code}` : ""}
              </li>
            ))}
          </ul>
          {task.verification.status !== "passed" ? (
            <p>Review this outcome before accepting or applying the result.</p>
          ) : null}
        </section>
      ) : null}

      <section className="delegation-next" aria-label="Next principal action">
        <span>Next principal action</span>
        <p>{task.next_action}</p>
      </section>

      {task.recent_activity?.length ? (
        <details className="delegation-activity">
          <summary>Recent activity</summary>
          <ol>
            {task.recent_activity.map((event) => (
              <li key={event.id}>
                <span>{statusLabel(event.type)}</span>
                <time dateTime={event.at}>{event.at}</time>
              </li>
            ))}
          </ol>
        </details>
      ) : null}

      {task.trace_artifacts?.length ? (
        <section className="delegation-traces" aria-label="ACP traces">
          <h3>ACP traces</h3>
          <p>Local wire captures can contain prompts, tool inputs and outputs.</p>
          <ul>
            {task.trace_artifacts.map((artifact) => (
              <li key={artifact}>
                <TraceDisclosure taskID={task.id} artifact={artifact} />
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </article>
  );
}

function TraceDisclosure({ taskID, artifact }: { taskID: string; artifact: string }) {
  const [open, setOpen] = useState(false);
  const attempt = artifact.slice("trace-".length);
  const query = new URLSearchParams({ task: taskID, artifact });
  const src = `/api/delegation/trace?${query.toString()}`;
  const handleToggle = (event: SyntheticEvent<HTMLDetailsElement>) => {
    setOpen(event.currentTarget.open);
  };

  return (
    <details className="delegation-trace" onToggle={handleToggle}>
      <summary>Attempt {attempt}</summary>
      {open ? (
        <iframe title={`ACP trace for attempt ${attempt}`} src={src} sandbox="" loading="lazy" />
      ) : null}
    </details>
  );
}

function Metadata({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: ReactNode;
  mono?: boolean;
}) {
  return (
    <div>
      <dt>{label}</dt>
      <dd className={mono ? "delegation-metadata__value--mono" : undefined}>{value}</dd>
    </div>
  );
}

function agentLabel(task: DelegatedTask) {
  if (!task.agent) return "";
  return task.model ? `${task.agent} · ${task.model}` : task.agent;
}

function statusLabel(status: string) {
  return status.replaceAll("_", " ");
}

function statusTone(status: string) {
  switch (status) {
    case "queued":
      return "queued";
    case "running":
      return "running";
    case "intervention_required":
    case "cancelling":
      return "attention";
    case "result_ready":
      return "result";
    case "accepted":
    case "completed":
      return "complete";
    case "failed":
      return "failed";
    case "discarded":
      return "neutral";
    default:
      return "neutral";
  }
}

function statusIcon(status: string) {
  switch (status) {
    case "queued":
      return "schedule";
    case "running":
      return "play_circle";
    case "intervention_required":
      return "warning";
    case "cancelling":
      return "pending";
    case "result_ready":
      return "rate_review";
    case "accepted":
      return "verified";
    case "completed":
      return "task_alt";
    case "failed":
      return "error";
    case "cancelled":
    case "discarded":
      return "cancel";
    default:
      return "help";
  }
}
