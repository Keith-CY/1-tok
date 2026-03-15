// Carrier job status display utilities.
import type { JobState } from "@1tok/contracts";

interface JobStateMeta {
  label: string;
  emoji: string;
  color: string;
}

const stateMeta: Record<JobState, JobStateMeta> = {
  pending: { label: "Pending", emoji: "⏳", color: "text-gray-500" },
  running: { label: "Running", emoji: "🔄", color: "text-blue-500" },
  completed: { label: "Completed", emoji: "✅", color: "text-green-600" },
  failed: { label: "Failed", emoji: "❌", color: "text-red-500" },
  cancelled: { label: "Cancelled", emoji: "🚫", color: "text-gray-400" },
};

export function getJobStateMeta(state: JobState): JobStateMeta {
  return stateMeta[state] ?? { label: state, emoji: "❓", color: "text-gray-500" };
}

export function formatJobState(state: JobState): string {
  const meta = getJobStateMeta(state);
  return `${meta.emoji} ${meta.label}`;
}

export function formatProgress(step: number, total: number): string {
  if (total === 0) return "0%";
  const pct = Math.round((step / total) * 100);
  const bar = "█".repeat(Math.round(pct / 10)) + "░".repeat(10 - Math.round(pct / 10));
  return `${bar} ${pct}%`;
}
