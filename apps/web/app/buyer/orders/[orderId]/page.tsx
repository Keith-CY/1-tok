import Link from "next/link";
import { RiArrowLeftLine, RiAuctionLine, RiFlashlightLine, RiPriceTag3Line, RiTimeLine } from "react-icons/ri";

import { formatMoney } from "@1tok/contracts";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { DeliveryReportPanel } from "@/components/delivery-report-panel";
import { SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { getOrders } from "@/lib/api";
import { requirePortalViewer } from "@/lib/viewer";

export const metadata = {
  title: "Buyer Order",
};

export const dynamic = "force-dynamic";

export default async function OrderDetailPage({ params }: { params: Promise<{ orderId: string }> }) {
  const { orderId } = await params;
  const viewer = await requirePortalViewer("buyer", `/buyer/orders/${orderId}`);
  const orders = await getOrders({ authToken: viewer.token, requireLive: true });
  const order = orders.find((candidate) => candidate.id === orderId);

  if (!order) {
    return (
      <WorkspaceShell
        role="buyer"
        title="Request not in current market"
        description="This record is not available in the current session, so the market view cannot be opened from here."
        actions={[
          { href: "/buyer", label: "Back to requests", icon: RiArrowLeftLine, variant: "outline" },
          { href: "/buyer/rfqs/create", label: "Post request" },
        ]}
      >
        <section className="rounded-md border border-border bg-card">
          <div className="grid gap-px bg-border lg:grid-cols-[1fr_1fr_1fr_1fr]">
            <MetricStrip
              icon={RiAuctionLine}
              label="Market status"
              value="Unavailable"
              detail="This request is not visible in the current market view."
            />
            <MetricStrip
              icon={RiFlashlightLine}
              label="Record visibility"
              value="Missing"
              detail="The current buyer session cannot reopen this service record."
            />
            <MetricStrip
              icon={RiPriceTag3Line}
              label="Best next step"
              value="Back"
              detail="Return to the live request board and reopen from there."
            />
            <MetricStrip
              icon={RiTimeLine}
              label="Fallback"
              value="Post new"
              detail="If the work is gone, create a new priced request."
            />
          </div>
        </section>

        <section className="grid gap-6 xl:grid-cols-[1.15fr_0.85fr] xl:items-start">
          <SectionCard
            eyebrow="Unavailable record"
            title="This request is not in the current book"
            description="The marketplace can only show records that still belong to the active buyer session and current environment."
          >
            <div className="grid gap-3">
              <Card className="market-card p-5">
                <div className="text-xs font-medium text-primary">What this means</div>
                <p className="mt-2 text-sm leading-7 text-muted-foreground">
                  The pricing or delivery record you tried to open is not available from this session anymore.
                </p>
              </Card>
              <Card className="market-card p-5">
                <div className="text-xs font-medium text-primary">What to do next</div>
                <p className="mt-2 text-sm leading-7 text-muted-foreground">
                  Go back to your request board, reopen a live request from the list, or post a new request into the market.
                </p>
              </Card>
            </div>
          </SectionCard>

          <SectionCard
            eyebrow="Next move"
            title="Return to a live market path"
            description="Keep the buyer flow moving instead of stopping on an empty system page."
          >
            <div className="grid gap-3">
              <Button asChild className="w-full justify-between">
                <Link href="/buyer">
                  Open request board
                  <RiArrowLeftLine className="size-4" />
                </Link>
              </Button>
              <Button asChild variant="outline" className="w-full justify-between">
                <Link href="/buyer/rfqs/create">
                  Post new request
                  <RiArrowLeftLine className="size-4" />
                </Link>
              </Button>
            </div>
          </SectionCard>
        </section>
      </WorkspaceShell>
    );
  }

  const totalBudget = order.milestones.reduce((sum, milestone) => sum + milestone.budgetCents, 0);
  const totalSpent = order.milestones.reduce(
    (sum, milestone) =>
      sum +
      milestone.settledCents +
      (Array.isArray(milestone.usageCharges)
        ? milestone.usageCharges.reduce((usage, charge) => usage + charge.amountCents, 0)
        : 0),
    0,
  );
  const runningMilestone = order.milestones.find((milestone) => milestone.state === "running") ?? order.milestones[0] ?? null;

  return (
    <WorkspaceShell
      role="buyer"
      title={`Closed on price · ${order.id}`}
      description="The pricing phase is over. This page only shows delivery progress, spend, and what needs your attention."
      status={orderStatusLabel(order.status)}
      actions={[{ href: "/buyer", label: "Back to requests", icon: RiArrowLeftLine, variant: "outline" }]}
    >
      <section className="rounded-md border border-border bg-card">
        <div className="grid gap-px bg-border lg:grid-cols-[1fr_1fr_1fr_1fr]">
          <MetricStrip
            icon={RiAuctionLine}
            label="Market status"
            value="Awarded"
            detail="The client selected a provider and pricing closed."
          />
          <MetricStrip
            icon={RiPriceTag3Line}
            label="Budget"
            value={formatMoney(totalBudget)}
            detail="Total across active milestones."
          />
          <MetricStrip
            icon={RiFlashlightLine}
            label="Spent so far"
            value={formatMoney(totalSpent)}
            detail="Usage and settled amount combined."
          />
          <MetricStrip
            icon={RiTimeLine}
            label="Milestones"
            value={`${order.milestones.length}`}
            detail="Stages remaining in delivery."
          />
        </div>
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr] xl:items-start">
        <SectionCard
          eyebrow="Delivery timeline"
          title="Milestones now in motion"
          description="Once the request is awarded, the delivery timeline becomes the only thing that matters."
        >
          <div className="space-y-4">
            {order.milestones.map((milestone) => {
              const usageCharges = Array.isArray(milestone.usageCharges) ? milestone.usageCharges : [];
              const spent = usageCharges.reduce((sum, charge) => sum + charge.amountCents, 0) + milestone.settledCents;
              const usage = milestone.budgetCents > 0 ? Math.min((spent / milestone.budgetCents) * 100, 100) : 0;
              const deliveryNote = milestone.summary?.trim() ?? "";

              return (
                <div key={milestone.id} className="market-card p-5">
                  <div className="grid gap-4 md:grid-cols-[1.4fr_0.72fr_0.72fr] md:items-start">
                    <div className="min-w-0 space-y-2">
                      <div className="text-xs font-medium text-primary">{milestoneStatusLabel(milestone.state)}</div>
                      <h3 className="text-lg font-semibold leading-tight tracking-tight break-words text-balance">{milestone.title}</h3>
                    </div>
                    <TimelineMetric label="Budget" value={formatMoney(milestone.budgetCents)} />
                    <TimelineMetric label="Spent" value={formatMoney(spent)} />
                  </div>
                  <div className="mt-4 space-y-3">
                    <Progress value={usage} />
                    <div className="text-sm text-muted-foreground">{usage.toFixed(0)}% used</div>
                  </div>
                  {deliveryNote ? <DeliveryReportPanel summary={deliveryNote} /> : null}
                </div>
              );
            })}
          </div>
        </SectionCard>

        <div className="space-y-6">
          <SectionCard
            eyebrow="Current state"
            title="What the buyer should watch"
            description="This rail keeps the active delivery signal clear without dragging the market UI back into admin mode."
          >
            <div className="grid gap-3 sm:grid-cols-2">
              <SpotMetric label="Status" value={orderStatusLabel(order.status)} />
              <SpotMetric label="Provider" value={order.providerOrgId} />
              <SpotMetric label="Funding mode" value={order.fundingMode} />
              <SpotMetric label="Current milestone" value={runningMilestone ? runningMilestone.title : "None"} />
            </div>
          </SectionCard>

          <SectionCard
            eyebrow="Next action"
            title="What needs attention now"
            description="If delivery is moving, there is usually nothing else to do."
          >
            <div className="grid gap-3">
              <Card className="market-card p-5">
                <div className="text-xs font-medium text-primary">Current guidance</div>
                <p className="mt-2 text-sm leading-7 text-muted-foreground">
                  {order.status === "awaiting_budget"
                    ? "This order is waiting for a budget confirmation or scope adjustment."
                    : "Delivery is progressing through milestones. No extra action is needed right now."}
                </p>
              </Card>
              <Button asChild variant="outline" className="w-full justify-between">
                <Link href="/buyer">
                  Back to requests
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
      <div className="mt-2 font-mono text-xl font-semibold leading-tight tracking-tight tabular-nums break-words text-foreground">
        {value}
      </div>
    </div>
  );
}

function SpotMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-border bg-secondary/50 px-4 py-4">
      <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{label}</div>
      <div className="mt-2 break-words font-mono text-2xl font-semibold tracking-tight tabular-nums text-foreground">{value}</div>
    </div>
  );
}

function orderStatusLabel(value: string) {
  if (value === "awaiting_budget") return "Waiting on budget";
  if (value === "awaiting_payment_rail") return "Waiting on payment rail";
  if (value === "running") return "In delivery";
  return value;
}

function milestoneStatusLabel(value: string) {
  return value === "settled" ? "Settled" : value === "paused" ? "Paused" : value === "running" ? "Running" : value;
}
