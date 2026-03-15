// Currency/amount formatting utilities.

/**
 * Formats cents as a dollar amount.
 * @example formatCents(1500) → "$15.00"
 */
export function formatCents(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

/**
 * Formats cents as a compact amount.
 * @example formatCentsCompact(150000) → "$1.5k"
 */
export function formatCentsCompact(cents: number): string {
  const dollars = cents / 100;
  if (dollars >= 1_000_000) return `$${(dollars / 1_000_000).toFixed(1)}M`;
  if (dollars >= 1_000) return `$${(dollars / 1_000).toFixed(1)}k`;
  return formatCents(cents);
}

/**
 * Formats a milestone budget usage as a percentage.
 * @example formatBudgetUsage(500, 1000) → "50%"
 */
export function formatBudgetUsage(spentCents: number, budgetCents: number): string {
  if (budgetCents === 0) return "—";
  const pct = Math.round((spentCents / budgetCents) * 100);
  return `${pct}%`;
}

/**
 * Returns a color class for budget usage.
 */
export function budgetUsageColor(spentCents: number, budgetCents: number): string {
  if (budgetCents === 0) return "text-gray-400";
  const ratio = spentCents / budgetCents;
  if (ratio >= 1) return "text-red-600";
  if (ratio >= 0.8) return "text-orange-500";
  if (ratio >= 0.5) return "text-yellow-500";
  return "text-green-600";
}
