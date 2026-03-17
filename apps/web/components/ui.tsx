interface StatusBadgeProps {
  status: string;
}

const statusMap: Record<string, string> = {
  running: "status-badge--running",
  pending: "status-badge--pending",
  settled: "status-badge--settled",
  open: "status-badge--open",
  paused: "status-badge--pending",
  completed: "status-badge--settled",
  failed: "status-badge--disputed",
  disputed: "status-badge--disputed",
  awarded: "status-badge--running",
  rejected: "status-badge--settled",
};

export function StatusBadge({ status }: StatusBadgeProps) {
  const className = statusMap[status] ?? "status-badge--pending";
  return <span className={`status-badge ${className}`}>{status}</span>;
}

interface ProgressBarProps {
  current: number;
  total: number;
  tone?: "default" | "warning" | "danger";
}

export function ProgressBar({ current, total, tone }: ProgressBarProps) {
  const pct = total > 0 ? Math.min((current / total) * 100, 100) : 0;
  const modifier =
    tone === "danger"
      ? " progress-bar__fill--danger"
      : tone === "warning"
        ? " progress-bar__fill--warning"
        : "";

  return (
    <div className="progress-bar" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}>
      <div className={`progress-bar__fill${modifier}`} style={{ width: `${pct}%` }} />
    </div>
  );
}

interface EmptyStateProps {
  icon?: string;
  message: string;
  actionLabel?: string;
  actionHref?: string;
}

export function EmptyState({ icon = "📭", message, actionLabel, actionHref }: EmptyStateProps) {
  return (
    <div className="empty-state">
      <span className="empty-state__icon">{icon}</span>
      <span>{message}</span>
      {actionLabel && actionHref ? <a className="empty-state__action" href={actionHref}>{actionLabel}</a> : null}
    </div>
  );
}
