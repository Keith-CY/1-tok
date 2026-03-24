"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { RequestBudgetField } from "@/components/request-budget-field";
import { RequestDeadlineField } from "@/components/request-deadline-field";
import { Textarea } from "@/components/ui/textarea";
import { Field, SectionCard } from "@/components/workspace-shell";

export function CreateRFQWorkspace() {
  const [title, setTitle] = useState("");
  const [budgetDollars, setBudgetDollars] = useState("");
  const [dateValue, setDateValue] = useState(() => formatUTCDateInput(new Date()));
  const [timeValue, setTimeValue] = useState("14:00");

  const previewTitle = title.trim() || "Research 5 AI phone agents for carrier support";
  const previewBudget = useMemo(() => formatBudgetPreview(budgetDollars), [budgetDollars]);
  const previewAwardDate = useMemo(() => formatAwardDatePreview(dateValue), [dateValue]);
  const previewAwardTime = timeValue || "14:00";

  return (
    <section className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr] xl:items-start">
      <SectionCard
        eyebrow="Request draft"
        title="Core listing fields"
        description="Keep it simple. A provider should understand the research task, price, and timing in one read."
      >
        <form action="/buyer/rfqs" method="post" className="space-y-6">
          <input type="hidden" name="category" value="service-request" />

          <div className="grid gap-4">
            <Field label="Request title" hint="Use a short title for a research, benchmarking, or market-scan task.">
              <Input
                name="title"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder="Research 5 AI phone agents for carrier support"
                className="min-h-[4.5rem] bg-card px-4 py-4 text-base font-medium tracking-[0.01em] shadow-[0_20px_40px_rgba(0,0,0,0.04)] placeholder:text-muted-foreground/80 focus-visible:border-b-accent"
                required
              />
            </Field>

            <div className="grid gap-3">
              <RequestBudgetField name="budgetCents" value={budgetDollars} onValueChange={setBudgetDollars} />
              <RequestDeadlineField
                name="responseDeadlineAt"
                dateValue={dateValue}
                timeValue={timeValue}
                onDateValueChange={setDateValue}
                onTimeValueChange={setTimeValue}
              />
            </div>
          </div>

          <Accordion type="single" collapsible>
            <AccordionItem value="more">
              <AccordionTrigger>Optional scope notes</AccordionTrigger>
              <AccordionContent>
                <Field label="Scope notes" hint="For the demo, stick to research context that changes scope, deadline, or output.">
                  <Textarea
                    name="scope"
                    rows={5}
                    placeholder="Example: compare 5 vendors, summarize pricing and strengths, and deliver a short recommendation memo within 2 hours."
                  />
                </Field>
              </AccordionContent>
            </AccordionItem>
          </Accordion>

          <div className="flex flex-wrap gap-3">
            <Button type="submit">Post request</Button>
            <Button asChild variant="outline">
              <Link href="/buyer">Cancel</Link>
            </Button>
          </div>
        </form>
      </SectionCard>

      <div className="space-y-6">
        <SectionCard
          eyebrow="Live listing preview"
          title="How providers will read this"
          description="Budget first, then current timing. Everything else is secondary."
        >
          <div className="bg-card px-5 py-5 shadow-[0_20px_40px_rgba(0,0,0,0.06)]">
            <div className="space-y-5">
              <div className="space-y-3">
                <div className="text-xs font-medium uppercase tracking-[0.16em] text-primary">Preview</div>
                <h3 className="text-xl font-semibold leading-tight tracking-tight text-balance text-foreground">{previewTitle}</h3>
                <p className="text-sm leading-7 text-muted-foreground">
                  Open market listing with a posted budget and a visible award window.
                </p>
              </div>

              <div className="grid gap-px bg-border sm:grid-cols-2">
                <PreviewMetric label="Budget" value={previewBudget} detail="posted by buyer" />
                <PreviewMetric label="Current low" value="Open" detail="waiting for first quote" />
                <PreviewMetric label="Award by" value={previewAwardDate} detail={`${previewAwardTime} JST`} />
                <PreviewMetric label="Proposals" value="0 live" detail="market not priced yet" />
              </div>
            </div>
          </div>
        </SectionCard>

        <SectionCard
          eyebrow="Market rules"
          title="What gets proposals faster"
          description="For the demo, research requests are the cleanest way to show pricing and award flow."
        >
          <div className="grid gap-3">
            <Card className="market-card p-5">
              <div className="text-xs font-medium text-primary">Lead with the number</div>
              <p className="mt-2 text-sm leading-7 text-muted-foreground">
                A real budget pulls in providers who are ready to compete on price.
              </p>
            </Card>
            <Card className="market-card p-5">
              <div className="text-xs font-medium text-primary">Name the work clearly</div>
              <p className="mt-2 text-sm leading-7 text-muted-foreground">
                Listing-style titles are priced faster than vague briefs.
              </p>
            </Card>
            <Card className="market-card p-5">
              <div className="text-xs font-medium text-primary">Make timing explicit</div>
              <p className="mt-2 text-sm leading-7 text-muted-foreground">
                Delivery pressure changes price. Put the deadline in the market.
              </p>
            </Card>
          </div>
        </SectionCard>
      </div>
    </section>
  );
}

function PreviewMetric({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="flex min-h-[7.25rem] min-w-0 flex-col justify-between bg-[var(--surface-lowest)] px-4 py-4">
      <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{label}</div>
      <div className="mt-3 font-mono text-xl font-semibold leading-tight tracking-tight tabular-nums break-words text-foreground">
        {value}
      </div>
      <div className="mt-3 text-xs leading-6 text-muted-foreground">{detail}</div>
    </div>
  );
}

function formatBudgetPreview(value: string): string {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return "$680";
  }

  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: parsed % 1 === 0 ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(parsed);
}

function formatAwardDatePreview(value: string): string {
  if (!value) {
    return "Mar 24";
  }

  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  }).format(new Date(`${value}T00:00:00Z`));
}

function formatUTCDateInput(value: Date): string {
  const year = value.getUTCFullYear();
  const month = String(value.getUTCMonth() + 1).padStart(2, "0");
  const day = String(value.getUTCDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}
