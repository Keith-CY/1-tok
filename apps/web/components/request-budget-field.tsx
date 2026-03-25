"use client";

import { useMemo, useState } from "react";
import { Input } from "@/components/ui/input";

export function dollarsInputToCents(value: string): string {
  const normalized = value.trim();
  if (!normalized) {
    return "";
  }

  const parsed = Number(normalized);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return "";
  }

  return String(Math.round(parsed * 100));
}

export function RequestBudgetField({
  name,
  value,
  onValueChange,
}: {
  name: string;
  value?: string;
  onValueChange?: (value: string) => void;
}) {
  const [internalBudgetDollars, setInternalBudgetDollars] = useState("");
  const budgetDollars = value ?? internalBudgetDollars;
  const setBudgetDollars = onValueChange ?? setInternalBudgetDollars;

  const budgetCents = useMemo(() => dollarsInputToCents(budgetDollars), [budgetDollars]);

  return (
    <div className="grid gap-3 sm:grid-cols-[9rem_minmax(0,1fr)] sm:items-center">
      <input type="hidden" name={name} value={budgetCents} />

      <div className="text-[11px] font-bold uppercase tracking-[0.18em] text-foreground/88">Budget</div>

      <div className="flex min-h-[4.25rem] items-center gap-4 bg-background px-4 py-3 transition-colors duration-150 focus-within:bg-card">
        <div className="font-mono text-[1.5rem] font-semibold tracking-tight text-accent">$</div>
        <Input
          type="number"
          inputMode="decimal"
          min="0.01"
          step="0.01"
          value={budgetDollars}
          onChange={(event) => setBudgetDollars(event.target.value)}
          placeholder="680.00"
          aria-label="Budget in US dollars"
          className="min-h-0 border-0 bg-transparent px-0 py-0 font-mono text-[1.5rem] font-semibold tracking-tight tabular-nums placeholder:font-mono placeholder:text-muted-foreground focus-visible:border-0 focus-visible:bg-transparent"
          required
        />
        <div className="text-[11px] font-bold uppercase tracking-[0.18em] text-muted-foreground">USD</div>
      </div>
    </div>
  );
}
