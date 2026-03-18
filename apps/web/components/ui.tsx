import { Glyph, type GlyphName } from "./portal-visuals";

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

export function InfoIcon({ label }: { label: string }) {
  return (
    <span className="info-dot" aria-label={label} title={label}>
      <Glyph name="info" className="info-dot__icon" />
    </span>
  );
}

interface PanelHeaderProps {
  icon: GlyphName;
  tag: string;
  title: string;
  info?: string;
}

export function PanelHeader({ icon, tag, title, info }: PanelHeaderProps) {
  return (
    <>
      <span className="tag tag--icon">
        <Glyph name={icon} className="tag__icon" />
        {tag}
        {info ? <InfoIcon label={info} /> : null}
      </span>
      <h3 className="panel-heading">
        <Glyph name={icon} className="panel-heading__icon" />
        <span>{title}</span>
      </h3>
    </>
  );
}

interface FieldLabelProps {
  icon: GlyphName;
  label: string;
  info?: string;
}

export function FieldLabel({ icon, label, info }: FieldLabelProps) {
  return (
    <span className="field-label">
      <span className="field-label__glyph">
        <Glyph name={icon} className="field-label__glyph-icon" />
      </span>
      <span className="field-label__text">{label}</span>
      {info ? <InfoIcon label={info} /> : null}
    </span>
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
      <div className="empty-state__icon-wrap">
        <span className="empty-state__icon">{icon}</span>
      </div>
      <div className="empty-state__body">
        <span className="empty-state__message">{message}</span>
        {actionLabel && actionHref ? (
          <a className="empty-state__action" href={actionHref}>
            {actionLabel}
          </a>
        ) : null}
      </div>
    </div>
  );
}
