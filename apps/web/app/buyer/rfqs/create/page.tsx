import Link from "next/link";
import { RiArrowLeftLine, RiAuctionLine, RiFlashlightLine, RiPriceTag3Line, RiTimeLine } from "react-icons/ri";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Field, SectionCard, WorkspaceShell } from "@/components/workspace-shell";

export const dynamic = "force-dynamic";

export default async function CreateRFQPage() {
  return (
    <WorkspaceShell
      role="buyer"
      title="Post a priced request"
      description="Publish the budget, keep the scope tight, and let providers compete into the number."
      actions={[{ href: "/buyer", label: "Back to requests", icon: RiArrowLeftLine, variant: "outline" }]}
    >
      <section className="rounded-md border border-border bg-card">
        <div className="grid gap-px bg-border lg:grid-cols-[1fr_1fr_1fr_1fr]">
          <MetricStrip
            icon={RiFlashlightLine}
            label="Listing mode"
            value="Open market"
            detail="Providers see the request as soon as it is posted."
          />
          <MetricStrip
            icon={RiPriceTag3Line}
            label="Budget"
            value="Required"
            detail="A clear budget pulls real pricing into the request."
          />
          <MetricStrip
            icon={RiTimeLine}
            label="Delivery window"
            value="Required"
            detail="Timing changes price, so it has to be explicit."
          />
          <MetricStrip
            icon={RiAuctionLine}
            label="Award path"
            value="Low wins"
            detail="You compare proposals on price and timing before award."
          />
        </div>
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr] xl:items-start">
        <SectionCard
          eyebrow="Request draft"
          title="Core listing fields"
          description="Keep this page tight. Everything here should change whether a provider prices the request."
        >
          <form action="/buyer/rfqs" method="post" className="space-y-6">
            <input type="hidden" name="category" value="service-request" />

            <div className="grid gap-4">
              <Field label="Request title" hint="Write it like a market listing, not a meeting note.">
                <Input name="title" placeholder="Carrier dispute triage package" required />
              </Field>

              <div className="grid gap-4 md:grid-cols-2">
                <Field label="Budget" hint="Enter the full request amount in dollars.">
                  <Input name="budgetDollars" type="number" min="0.01" step="0.01" inputMode="decimal" placeholder="68.00" required />
                </Field>
                <Field label="Delivery window" hint="This is the date providers will price against.">
                  <Input name="responseDeadlineAt" type="datetime-local" required />
                </Field>
              </div>
            </div>

            <Accordion type="single" collapsible>
              <AccordionItem value="more">
                <AccordionTrigger>Optional scope notes</AccordionTrigger>
                <AccordionContent>
                  <Field label="Scope notes" hint="Only add context that should change price, timing, or delivery shape.">
                    <Textarea
                      name="scope"
                      rows={5}
                      placeholder="Example: review a live carrier dispute, map the missing evidence, and propose the response package for this week."
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
            <div className="space-y-3">
              <div className="market-row">
                <div className="grid gap-4 md:grid-cols-[1.45fr_0.72fr_0.72fr_0.72fr] md:items-center">
                  <div className="min-w-0 space-y-2">
                    <div className="text-xs font-medium text-primary">Preview</div>
                    <h3 className="text-lg font-semibold leading-tight tracking-tight break-words text-balance">
                      Carrier dispute triage package
                    </h3>
                  </div>
                  <PreviewMetric label="Budget" value="$68.00" />
                  <PreviewMetric label="Current low" value="Open" />
                  <div className="space-y-1 text-sm text-muted-foreground">
                    <div>0 live proposals</div>
                    <div>Mar 22</div>
                  </div>
                </div>
              </div>
            </div>
          </SectionCard>

          <SectionCard
            eyebrow="Market rules"
            title="What gets proposals faster"
            description="The strongest listings are concrete, priced, and easy to scan."
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
    </WorkspaceShell>
  );
}

function MetricStrip({
  icon: Icon,
  label,
  value,
  detail,
}: {
  icon: typeof RiFlashlightLine;
  label: string;
  value: string;
  detail: string;
}) {
  return (
    <div className="space-y-2 bg-card px-5 py-5">
      <div className="inline-flex items-center gap-2 text-[11px] uppercase tracking-[0.16em] text-muted-foreground">
        <Icon className="size-4 text-primary" />
        {label}
      </div>
      <div className="font-mono text-3xl font-semibold tracking-tight tabular-nums text-foreground">{value}</div>
      <div className="text-sm text-muted-foreground">{detail}</div>
    </div>
  );
}

function PreviewMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{label}</div>
      <div className="mt-2 font-mono text-xl font-semibold leading-tight tracking-tight tabular-nums break-words text-foreground">
        {value}
      </div>
    </div>
  );
}
