"use client";

import { useMemo, useState } from "react";
import { RiArrowLeftSLine, RiArrowRightSLine, RiCalendarLine } from "react-icons/ri";

import { Input } from "@/components/ui/input";

type CalendarCell = {
  isoDate: string;
  dayOfMonth: number;
  inMonth: boolean;
};

const weekdayLabels = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

export function combineDeadlineValue(dateValue: string, timeValue: string): string {
  if (!dateValue || !timeValue) {
    return "";
  }
  return `${dateValue}T${timeValue}`;
}

export function RequestDeadlineField({
  name,
  dateValue: controlledDateValue,
  timeValue: controlledTimeValue,
  onDateValueChange,
  onTimeValueChange,
}: {
  name: string;
  dateValue?: string;
  timeValue?: string;
  onDateValueChange?: (value: string) => void;
  onTimeValueChange?: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [internalDateValue, setInternalDateValue] = useState(() => formatISODate(new Date()));
  const [internalTimeValue, setInternalTimeValue] = useState("14:00");
  const [visibleMonth, setVisibleMonth] = useState(() => {
    const now = new Date();
    return new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1));
  });
  const dateValue = controlledDateValue ?? internalDateValue;
  const timeValue = controlledTimeValue ?? internalTimeValue;
  const setDateValue = onDateValueChange ?? setInternalDateValue;
  const setTimeValue = onTimeValueChange ?? setInternalTimeValue;

  const monthLabel = useMemo(
    () =>
      new Intl.DateTimeFormat("en-US", {
        month: "long",
        year: "numeric",
        timeZone: "UTC",
      }).format(visibleMonth),
    [visibleMonth],
  );

  const selectedDateLabel = dateValue
    ? new Intl.DateTimeFormat("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
        timeZone: "UTC",
      }).format(new Date(`${dateValue}T00:00:00Z`))
    : "Select date";

  const cells = useMemo(() => buildCalendarCells(visibleMonth), [visibleMonth]);
  const combinedValue = combineDeadlineValue(dateValue, timeValue);

  return (
    <div className="grid gap-3 sm:grid-cols-[9rem_minmax(0,1fr)] sm:items-center">
      <input type="hidden" name={name} value={combinedValue} required />

      <div className="text-[11px] font-bold uppercase tracking-[0.18em] text-foreground/88">Delivery window</div>

      <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_9rem]">
        <div className="relative">
          <button
            type="button"
            className="grid min-h-[4.25rem] w-full grid-cols-[auto_minmax(0,1fr)] items-center gap-3 overflow-hidden bg-background px-4 py-3 text-left text-foreground transition-colors hover:bg-card"
            onClick={() => setOpen((value) => !value)}
          >
            <RiCalendarLine className="size-4 shrink-0 text-accent" />
            <span className={`${dateValue ? "text-foreground" : "text-muted-foreground"} min-w-0 truncate text-base font-medium`}>
              {selectedDateLabel}
            </span>
          </button>

          {open ? (
            <div className="absolute left-0 z-20 mt-3 w-full min-w-[20rem] bg-card p-4 shadow-[0_20px_40px_rgba(0,0,0,0.08)]">
              <div className="flex items-center justify-between gap-3">
                <button
                  type="button"
                  className="flex size-9 items-center justify-center bg-secondary text-foreground transition-colors hover:bg-[var(--accent-weak)]"
                  onClick={() => setVisibleMonth(addMonthsUTC(visibleMonth, -1))}
                  aria-label="Previous month"
                >
                  <RiArrowLeftSLine className="size-4" />
                </button>
                <div className="text-sm font-semibold tracking-[0.01em] text-foreground">{monthLabel}</div>
                <button
                  type="button"
                  className="flex size-9 items-center justify-center bg-secondary text-foreground transition-colors hover:bg-[var(--accent-weak)]"
                  onClick={() => setVisibleMonth(addMonthsUTC(visibleMonth, 1))}
                  aria-label="Next month"
                >
                  <RiArrowRightSLine className="size-4" />
                </button>
              </div>

              <div className="mt-4 grid grid-cols-7 gap-2">
                {weekdayLabels.map((label) => (
                  <div key={label} className="px-2 py-1 text-center text-[10px] font-bold uppercase tracking-[0.16em] text-muted-foreground">
                    {label}
                  </div>
                ))}
                {cells.map((cell) => {
                  const selected = cell.isoDate === dateValue;
                  return (
                    <button
                      key={cell.isoDate}
                      type="button"
                      className={
                        selected
                          ? "flex h-10 items-center justify-center bg-[var(--accent-weak)] text-foreground"
                          : cell.inMonth
                              ? "flex h-10 items-center justify-center bg-secondary text-foreground transition-colors hover:bg-[var(--accent-weak)]"
                              : "flex h-10 items-center justify-center bg-background text-muted-foreground/70 transition-colors hover:bg-secondary"
                      }
                      onClick={() => {
                        setDateValue(cell.isoDate);
                        setOpen(false);
                      }}
                    >
                      {cell.dayOfMonth}
                    </button>
                  );
                })}
              </div>
            </div>
          ) : null}
        </div>

        <div className="flex min-h-[4.25rem] items-center border-x-0 border-t-0 border-b border-transparent bg-background px-4 py-3 focus-within:border-b-muted-foreground/50 focus-within:bg-card">
          <Input
            type="time"
            value={timeValue}
            onChange={(event) => setTimeValue(event.target.value)}
            aria-label="Award time"
            className="min-h-0 border-0 bg-transparent px-0 py-0 font-mono text-lg font-medium tracking-tight tabular-nums focus-visible:border-0 focus-visible:bg-transparent"
            required
          />
        </div>
      </div>
    </div>
  );
}

function buildCalendarCells(visibleMonth: Date): CalendarCell[] {
  const monthStart = new Date(Date.UTC(visibleMonth.getUTCFullYear(), visibleMonth.getUTCMonth(), 1));
  const gridStart = new Date(monthStart);
  gridStart.setUTCDate(gridStart.getUTCDate() - gridStart.getUTCDay());

  return Array.from({ length: 42 }, (_, index) => {
    const cellDate = new Date(gridStart);
    cellDate.setUTCDate(gridStart.getUTCDate() + index);
    return {
      isoDate: formatISODate(cellDate),
      dayOfMonth: cellDate.getUTCDate(),
      inMonth: cellDate.getUTCMonth() === visibleMonth.getUTCMonth(),
    };
  });
}

function formatISODate(value: Date): string {
  const year = value.getUTCFullYear();
  const month = String(value.getUTCMonth() + 1).padStart(2, "0");
  const day = String(value.getUTCDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function addMonthsUTC(value: Date, offset: number): Date {
  return new Date(Date.UTC(value.getUTCFullYear(), value.getUTCMonth() + offset, 1));
}
