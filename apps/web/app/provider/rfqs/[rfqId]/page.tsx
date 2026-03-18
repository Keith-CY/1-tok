import Link from "next/link";
import { RiArrowLeftLine, RiSendPlaneLine } from "react-icons/ri";

import { formatMoney } from "@1tok/contracts";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { DetailChip, Field, SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { getProviderRFQDetail } from "@/lib/api";
import { requirePortalViewer } from "@/lib/viewer";

export const dynamic = "force-dynamic";

export default async function ProviderRFQDetailsPage({ params }: { params: Promise<{ rfqId: string }> }) {
  const { rfqId } = await params;
  const viewer = await requirePortalViewer("provider", "/provider/rfqs");
  const detail = await getProviderRFQDetail({
    authToken: viewer.token,
    providerOrgId: viewer.membership.organization.id,
    rfqId,
    requireLive: true,
  });
  const rfq = detail?.rfq;
  const providerBid = detail?.providerBid ?? null;

  if (!rfq) {
    return (
      <WorkspaceShell
        role="provider"
        title="Request not found"
        description="This request may have closed or moved out of the live market."
        actions={[{ href: "/provider/rfqs", label: "Back to requests", icon: RiArrowLeftLine, variant: "outline" }]}
      >
        <Card className="border-dashed p-6 text-sm text-muted-foreground">This request is not available in the current environment.</Card>
      </WorkspaceShell>
    );
  }

  const suggestedQuote = providerBid?.quoteCents ?? Math.max(Math.floor(rfq.budgetCents * 0.8), 1000);
  const currentStatus = providerBid?.status === "awarded" ? "Awarded" : providerBid ? "Live proposal" : rfq.status === "open" ? "Open" : "Closed";
  const bidPayload = rfq as { bids?: Array<{ quoteCents: number }>; bidCount?: number };
  const bids = Array.isArray(bidPayload.bids) ? bidPayload.bids : [];
  const bidCount = typeof bidPayload.bidCount === "number" ? bidPayload.bidCount : Math.max(bids.length, providerBid ? 1 : 0);
  const lowestBid = bids.length > 0 ? Math.min(...bids.map((bid) => bid.quoteCents)) : providerBid?.quoteCents ?? null;
  const priceDelta = lowestBid !== null ? suggestedQuote - lowestBid : rfq.budgetCents - suggestedQuote;

  return (
    <WorkspaceShell
      role="provider"
      title={rfq.title}
      description="This is a live 1-tok request. Budget, current low proposal, and your price position should all be visible before you submit."
      status={currentStatus}
      actions={[
        { href: "/provider/rfqs", label: "Back to requests", icon: RiArrowLeftLine, variant: "outline" },
        { href: "/provider/proposals", label: "My proposals", variant: "outline" },
      ]}
    >
      <section className="grid gap-4 lg:grid-cols-[1.2fr_0.8fr_0.8fr_0.8fr]">
        <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
          <div className="text-xs text-muted-foreground">Budget</div>
          <div className="price-display mt-3">{formatMoney(rfq.budgetCents)}</div>
          <p className="mt-3 text-sm leading-7 text-muted-foreground text-pretty">
            The client anchored this request with a public budget. Your proposal competes against that number first.
          </p>
        </div>
        <DetailChip label="Current low proposal" value={lowestBid !== null ? formatMoney(lowestBid) : "Be first"} />
        <DetailChip label="Proposal count" value={String(bidCount)} />
        <DetailChip label="Delivery window" value={formatDate(rfq.responseDeadlineAt)} />
      </section>

      <section className="grid gap-6 xl:grid-cols-[0.96fr_1.04fr]">
        <SectionCard eyebrow="Request brief" title="Scope that affects pricing" description="Only the information that should change your proposal stays visible here.">
          <div className="rounded-lg border border-border bg-secondary/60 p-5 text-sm leading-7 text-muted-foreground">{rfq.scope}</div>
        </SectionCard>

        <SectionCard eyebrow="Proposal panel" title="Place your price in the market" description="Before you bid, you should know how your number compares to the current low proposal.">
          <div className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="rounded-md border border-border bg-secondary/60 px-4 py-3">
                <div className="text-xs text-muted-foreground">Suggested entry</div>
                <div className="mt-1 font-mono text-xl font-semibold tabular-nums text-foreground">{formatMoney(suggestedQuote)}</div>
              </div>
              <div className="rounded-md border border-border bg-secondary/60 px-4 py-3">
                <div className="text-xs text-muted-foreground">Price position</div>
                <div className="mt-1 text-sm font-medium text-foreground">{formatPricePosition(priceDelta, lowestBid === null)}</div>
              </div>
            </div>

            <form action={`/provider/rfqs/${rfq.id}/bids`} method="post" className="space-y-4">
              <Field label="Proposal price" hint="Enter the full proposal amount.">
                <Input
                  name="quoteCents"
                  type="number"
                  min="1"
                  step="1"
                  defaultValue={String(suggestedQuote)}
                  className="h-14 text-2xl font-mono font-semibold tabular-nums"
                  required
                />
              </Field>

              {providerBid ? (
                <div className="rounded-lg border border-primary/15 bg-primary/5 px-4 py-3 text-sm text-primary">
                  You already placed a live proposal at {formatMoney(providerBid.quoteCents)}.
                </div>
              ) : null}

              <Accordion type="single" collapsible>
                <AccordionItem value="note">
                  <AccordionTrigger>Proposal note</AccordionTrigger>
                  <AccordionContent>
                    <Field label="Note (optional)" hint="Only explain what makes your price stronger.">
                      <Textarea name="message" rows={4} placeholder="For example: includes two revision rounds and a three-day delivery window." />
                    </Field>
                  </AccordionContent>
                </AccordionItem>
              </Accordion>

              <div className="flex flex-wrap gap-3">
                <Button type="submit" disabled={rfq.status !== "open" || Boolean(providerBid)}>
                  <RiSendPlaneLine className="size-4" />
                  {providerBid ? "Proposal already submitted" : "Submit proposal"}
                </Button>
                <Button asChild variant="outline">
                  <Link href="/provider/rfqs">Maybe later</Link>
                </Button>
              </div>
            </form>
          </div>
        </SectionCard>
      </section>
    </WorkspaceShell>
  );
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric" }).format(new Date(value));
}

function formatPricePosition(delta: number, noBidsYet: boolean) {
  if (noBidsYet) return "No live proposals yet. You can set the first price.";
  if (delta === 0) return "You match the current low proposal.";
  if (delta < 0) return `You are ${formatMoney(Math.abs(delta))} below the current low.`;
  return `You are ${formatMoney(delta)} above the current low.`;
}
