import Link from "next/link";
import { RiArrowLeftLine, RiFlashlightLine, RiMoneyDollarCircleLine, RiPriceTag3Line, RiTimeLine } from "react-icons/ri";

import { formatMoney } from "@1tok/contracts";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { DeliveryReportPanel } from "@/components/delivery-report-panel";
import { DetailChip, SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { getProviderOrderDetail } from "@/lib/api";
import { requirePortalViewer } from "@/lib/viewer";

export const metadata = {
  title: "Provider Order",
};

export const dynamic = "force-dynamic";

export default async function ProviderOrderPage({ params }: { params: Promise<{ orderId: string }> }) {
  const { orderId } = await params;
  const viewer = await requirePortalViewer("provider", `/provider/orders/${orderId}`);
  const detail = await getProviderOrderDetail({
    authToken: viewer.token,
    providerOrgId: viewer.membership.organization.id,
    orderId,
    requireLive: true,
  });

  if (!detail) {
    return (
      <WorkspaceShell
        role="provider"
        title="Delivery not available"
        description="This delivery record is not available for the current provider session."
        actions={[
          { href: "/provider", label: "Back to marketplace", icon: RiArrowLeftLine, variant: "outline" },
          { href: "/provider/proposals", label: "My proposals", variant: "outline" },
        ]}
      >
        <Card className="border-dashed p-6 text-sm text-muted-foreground">This order is not visible in the current provider environment.</Card>
      </WorkspaceShell>
    );
  }

  const { order, rfq, fundingRecords } = detail;
  const totalBudget = order.milestones.reduce((sum, milestone) => sum + milestone.budgetCents, 0);
  const totalSettled = order.milestones.reduce(
    (sum, milestone) =>
      sum +
      milestone.settledCents +
      (Array.isArray(milestone.usageCharges)
        ? milestone.usageCharges.reduce((usage, charge) => usage + charge.amountCents, 0)
        : 0),
    0,
  );
  const allSettled = order.milestones.length > 0 && order.milestones.every((milestone) => milestone.state === "settled");
  const invoiceRecords = fundingRecords.filter((record) => record.kind === "invoice");
  const payoutRecords = fundingRecords.filter((record) => record.kind === "withdrawal" || record.kind === "provider_payout");
  const runningMilestone = order.milestones.find((milestone) => milestone.state === "running") ?? order.milestones[0] ?? null;
  const invoiceState = invoiceRecords[0]?.state ?? (allSettled ? "SETTLED" : "PENDING");
  const payoutState = payoutRecords[0]?.state ?? (allSettled ? "READY" : "NOT REQUESTED");

  return (
    <WorkspaceShell
      role="provider"
      title={rfq?.title ?? `Delivery ${order.id}`}
      description="This view tracks the work after price is accepted: milestone state, settlement, and payout readiness."
      status={providerDeliveryStatus(order.status, allSettled)}
      actions={[
        { href: "/provider/proposals", label: "Back to proposals", icon: RiArrowLeftLine, variant: "outline" },
        { href: "/provider", label: "Marketplace", variant: "outline" },
      ]}
    >
      <section className="rounded-md border border-border bg-card">
        <div className="grid gap-px bg-border lg:grid-cols-[1fr_1fr_1fr_1fr]">
          <MetricStrip
            icon={RiFlashlightLine}
            label="Delivery state"
            value={providerDeliveryStatus(order.status, allSettled)}
            detail={runningMilestone ? `${runningMilestone.title} is the current milestone.` : "No active milestone right now."}
          />
          <MetricStrip
            icon={RiPriceTag3Line}
            label="Contracted value"
            value={formatMoney(totalBudget)}
            detail="Budget committed when the request was awarded."
          />
          <MetricStrip
            icon={RiMoneyDollarCircleLine}
            label="Settled so far"
            value={formatMoney(totalSettled)}
            detail="Milestone settlement and usage charges combined."
          />
          <MetricStrip
            icon={RiTimeLine}
            label="Payout state"
            value={invoiceState}
            detail={payoutState === "NOT REQUESTED" ? "Invoice settled before payout starts." : `Payout ${payoutState}.`}
          />
        </div>
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.18fr_0.82fr] xl:items-start">
        <SectionCard
          eyebrow="Delivery timeline"
          title="Milestones and proof of completion"
          description="This is the provider-facing delivery rail. It keeps execution and payout in the same place."
        >
          <div className="space-y-4">
            {order.milestones.map((milestone) => {
              const usageCharges = Array.isArray(milestone.usageCharges) ? milestone.usageCharges : [];
              const spent = usageCharges.reduce((sum, charge) => sum + charge.amountCents, 0) + milestone.settledCents;
              const usage = milestone.budgetCents > 0 ? Math.min((spent / milestone.budgetCents) * 100, 100) : 0;
              const deliveryNote = milestone.summary?.trim() ?? "";

              return (
                <div key={milestone.id} className="market-card p-5">
                  <div className="grid gap-4 md:grid-cols-[1.45fr_0.75fr_0.75fr] md:items-start">
                    <div className="min-w-0 space-y-2">
                      <div className="text-xs font-medium text-primary">{milestoneStatusLabel(milestone.state)}</div>
                      <h3 className="text-lg font-semibold leading-tight tracking-tight break-words text-balance">{milestone.title}</h3>
                    </div>
                    <TimelineMetric label="Budget" value={formatMoney(milestone.budgetCents)} />
                    <TimelineMetric label="Settled" value={formatMoney(spent)} />
                  </div>
                  <div className="mt-4 space-y-3">
                    <Progress value={usage} />
                    <div className="text-sm text-muted-foreground">{usage.toFixed(0)}% consumed</div>
                  </div>
                  {deliveryNote ? <DeliveryReportPanel summary={deliveryNote} /> : null}
                </div>
              );
            })}
          </div>
        </SectionCard>

        <div className="space-y-6">
          <SectionCard
            eyebrow="Payout rail"
            title="Settlement and cash-out state"
            description="The provider should be able to see whether the work is still earning, already settled, or ready for payout."
          >
            <div className="grid gap-3 sm:grid-cols-2">
              <DetailChip label="Invoice state" value={invoiceState} />
              <DetailChip label="Payout state" value={payoutState} />
              <DetailChip label="Funding records" value={`${fundingRecords.length}`} />
              <DetailChip label="Current milestone" value={runningMilestone ? runningMilestone.title : "None"} />
            </div>
          </SectionCard>

          <SectionCard
            eyebrow="Completion summary"
            title={allSettled ? "Delivery completed" : "Delivery still in motion"}
            description="This panel is the final checkpoint before the provider leaves the order page."
          >
            <div className="grid gap-3">
              <Card className="market-card p-5">
                <div className="text-xs font-medium text-primary">Current read</div>
                <p className="mt-2 text-sm leading-7 text-muted-foreground">
                  {allSettled
                    ? "Every milestone on this order is settled. The provider has completed delivery and the payout rail is now the only thing left to watch."
                    : "The order is still live. Finish the current milestone and wait for settlement to move the payout rail forward."}
                </p>
              </Card>
              <Button asChild variant="outline" className="w-full justify-between">
                <Link href="/provider/proposals">
                  Back to proposals
                  <RiArrowLeftLine className="size-4" />
                </Link>
              </Button>
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

function TimelineMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{label}</div>
      <div className="mt-2 font-mono text-xl font-semibold leading-tight tracking-tight tabular-nums break-words text-foreground">{value}</div>
    </div>
  );
}

function providerDeliveryStatus(value: string, allSettled: boolean) {
  if (allSettled || value === "completed") return "Completed";
  if (value === "running") return "In delivery";
  if (value === "awaiting_budget") return "Waiting on budget";
  if (value === "awaiting_payment_rail") return "Waiting on payment rail";
  return value;
}

function milestoneStatusLabel(value: string) {
  if (value === "settled") return "Settled";
  if (value === "paused") return "Paused";
  if (value === "running") return "Running";
  return value;
}
