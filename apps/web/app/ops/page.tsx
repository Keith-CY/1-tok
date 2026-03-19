import Link from "next/link";
import { RiArrowRightUpLine, RiBankCardLine, RiCheckboxCircleLine, RiFundsLine, RiSearchEyeLine, RiShieldCheckLine, RiSparklingLine, RiTimeLine } from "react-icons/ri";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { SectionCard, StatCard, WorkspaceShell, Field } from "@/components/workspace-shell";
import { formatShortDate } from "@/lib/utils";
import { getOpsDashboardData } from "@/lib/api";
import { requirePortalViewer } from "@/lib/viewer";

export const dynamic = "force-dynamic";

export default async function OpsPage({
  searchParams,
}: {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
}) {
  const viewer = await requirePortalViewer("ops", "/ops");
  const data = await getOpsDashboardData({ authToken: viewer.token, requireLive: true });
  const params = await searchParams;
  const creditApproved = String(params?.creditApproved ?? "");
  const recommendedLimitCents = String(params?.recommendedLimitCents ?? "0");
  const creditReason = String(params?.creditReason ?? "");
  const resolvedDisputeId = String(params?.resolvedDisputeId ?? "");
  const disputeStatus = String(params?.disputeStatus ?? "");
  const error = String(params?.error ?? "");

  const openDisputes = data.disputes.filter((item) => item.status !== "resolved").slice(0, 3);

  return (
    <WorkspaceShell
      role="ops"
      title="Put human review at the top. Push everything else down a layer."
      description="The ops homepage stays opinionated: review queue first, credit panel second, treasury summary third. Dense audit surfaces move to secondary routes."
      status={`${data.summary.openDisputes} open disputes`}
      actions={[
        { href: "/ops/applications", label: "Review queue", icon: RiSearchEyeLine },
        { href: "/ops/disputes", label: "Disputes", icon: RiShieldCheckLine, variant: "outline" },
      ]}
    >
      <section className="grid gap-4 xl:grid-cols-4">
        <StatCard icon={RiSearchEyeLine} label="Needs review" value={`${data.pendingReviews.length + data.summary.openDisputes}`} detail="Combined manual queue across applications, treasury, and disputes." />
        <StatCard icon={RiFundsLine} label="Funding records" value={`${data.summary.fundingRecords}`} detail="Treasury records visible in the control plane." />
        <StatCard icon={RiCheckboxCircleLine} label="Settled invoices" value={`${data.summary.settledInvoices}`} detail="Settlement entries already cleared." tone="success" />
        <StatCard icon={RiTimeLine} label="Pending withdrawals" value={`${data.summary.pendingWithdrawals}`} detail="Cash movement still in flight." tone={data.summary.pendingWithdrawals > 0 ? "warning" : "default"} />
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.02fr_0.98fr]">
        <SectionCard eyebrow="Primary queue" title="Needs review now" description="Disputes and operational review items share one surface so ops starts from action, not from tabs.">
          <div className="space-y-3">
            {openDisputes.map((dispute) => (
              <div key={dispute.id} className="space-y-3 rounded-[26px] border border-border/70 bg-white/82 p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="font-medium text-foreground">{dispute.id}</h3>
                      <Badge variant="danger">open</Badge>
                    </div>
                    <p className="mt-1 text-sm text-muted-foreground">Order {dispute.orderId} · Milestone {dispute.milestoneId} · {formatShortDate(dispute.createdAt)}</p>
                    <p className="mt-2 text-sm text-foreground">{dispute.reason}</p>
                  </div>
                  <div className="font-display text-2xl tracking-[-0.04em] text-foreground">{dispute.refundCents}</div>
                </div>
                <div className="flex flex-wrap gap-2">
                  <form action={`/ops/disputes/${dispute.id}/resolve`} method="post">
                    <input type="hidden" name="resolution" value="refund approved" />
                    <Button type="submit" size="sm">Approve refund</Button>
                  </form>
                  <form action={`/ops/disputes/${dispute.id}/resolve`} method="post">
                    <input type="hidden" name="resolution" value="claim rejected" />
                    <Button type="submit" variant="outline" size="sm">Reject claim</Button>
                  </form>
                </div>
              </div>
            ))}
            {data.pendingReviews.map((review) => (
              <div key={review.id} className="rounded-[24px] border border-border/70 bg-secondary/45 p-4">
                <div className="font-medium text-foreground">{review.title}</div>
                <div className="mt-1 text-sm text-muted-foreground">{review.detail}</div>
              </div>
            ))}
          </div>
        </SectionCard>

        <SectionCard eyebrow="Decisioning" title="Run credit decision" description="Keep the default input set short. Advanced scoring factors stay collapsed until needed.">
          <form action="/ops/credits/decision" method="post" className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <Field label="Completed orders"><Input name="completedOrders" type="number" min="0" step="1" defaultValue="12" required /></Field>
              <Field label="Disputed orders"><Input name="disputedOrders" type="number" min="0" step="1" defaultValue={String(data.summary.openDisputes)} required /></Field>
            </div>
            <Field label="Lifetime spend"><Input name="lifetimeSpendCents" type="number" min="0" step="1" defaultValue="480000" required /></Field>
            <Accordion type="single" collapsible>
              <AccordionItem value="advanced-credit">
                <AccordionTrigger>Advanced scoring factors</AccordionTrigger>
                <AccordionContent>
                  <div className="grid gap-4 md:grid-cols-2">
                    <Field label="Successful payments"><Input name="successfulPayments" type="number" min="0" step="1" defaultValue="11" required /></Field>
                    <Field label="Failed payments"><Input name="failedPayments" type="number" min="0" step="1" defaultValue="1" required /></Field>
                  </div>
                </AccordionContent>
              </AccordionItem>
            </Accordion>
            <div className="flex flex-wrap gap-3">
              <Button type="submit">Run credit decision</Button>
              <Button asChild variant="outline">
                <Link href="/ops/disputes">Open disputes board</Link>
              </Button>
            </div>
          </form>
        </SectionCard>
      </section>

      <section className="grid gap-6 xl:grid-cols-[0.92fr_1.08fr]">
        <SectionCard eyebrow="Latest output" title="Decision + resolution" description="Small feedback cards keep the homepage readable after an action completes.">
          <div className="grid gap-3">
            <Card className="bg-white/82 p-5">
              <div className="flex items-center gap-3">
                <RiSparklingLine className="size-5 text-primary" />
                <div>
                  <div className="font-medium text-foreground">Credit result</div>
                  <div className="mt-1 text-sm text-muted-foreground">
                    {creditApproved === "true"
                      ? `Approved with recommended limit ${recommendedLimitCents} cents.`
                      : creditApproved === "false"
                        ? `Rejected. ${creditReason || "No reason supplied."}`
                        : "No fresh decision yet."}
                  </div>
                </div>
              </div>
            </Card>
            <Card className="bg-white/82 p-5">
              <div className="flex items-center gap-3">
                <RiShieldCheckLine className="size-5 text-primary" />
                <div>
                  <div className="font-medium text-foreground">Dispute action</div>
                  <div className="mt-1 text-sm text-muted-foreground">
                    {resolvedDisputeId
                      ? `Resolved ${resolvedDisputeId} with status ${disputeStatus || "resolved"}.`
                      : error
                        ? `The last dispute action returned ${error}.`
                        : "No dispute action has been recorded in this session."}
                  </div>
                </div>
              </div>
            </Card>
          </div>
        </SectionCard>

        <SectionCard eyebrow="Treasury + risk" title="Short posture strip" description="Only the top-level posture lives here. Full treasury browsing moves to secondary pages.">
          <div className="grid gap-3 md:grid-cols-3">
            {data.treasurySignals.map((signal) => (
              <Card key={signal.id} className="bg-white/82 p-5">
                <div className="text-[11px] uppercase tracking-[0.22em] text-muted-foreground">{signal.label}</div>
                <div className="mt-2 font-display text-3xl tracking-[-0.04em]">{signal.value}</div>
              </Card>
            ))}
          </div>
          <div className="grid gap-3 pt-2">
            {data.riskFeed.map((item) => (
              <div key={item.id} className="rounded-[24px] border border-border/70 bg-secondary/45 p-4">
                <div className="font-medium text-foreground">{item.title}</div>
                <div className="mt-1 text-sm text-muted-foreground">{item.detail}</div>
              </div>
            ))}
          </div>
        </SectionCard>
      </section>
    </WorkspaceShell>
  );
}
