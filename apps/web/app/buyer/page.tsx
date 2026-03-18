import Link from "next/link";
import { RiArrowRightUpLine, RiAuctionLine, RiPriceTag3Line, RiTimeLine } from "react-icons/ri";

import { formatMoney } from "@1tok/contracts";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { getBuyerDashboardData } from "@/lib/api";
import { requirePortalViewer } from "@/lib/viewer";

export const dynamic = "force-dynamic";

export default async function BuyerPage() {
  const viewer = await requirePortalViewer("buyer", "/buyer");
  const data = await getBuyerDashboardData({
    authToken: viewer.token,
    buyerOrgId: viewer.membership.organization.id,
    requireLive: true,
  });

  const openRequests = [...data.rfqBook]
    .filter((item) => item.status === "open")
    .sort((left, right) => right.budgetCents - left.budgetCents || Date.parse(left.responseDeadlineAt) - Date.parse(right.responseDeadlineAt));
  const topRequest = openRequests[0] ?? null;
  const liveFloor = openRequests
    .flatMap((item) => item.bids.map((bid) => bid.quoteCents))
    .sort((left, right) => left - right)[0] ?? null;
  const closingSoon = openRequests.filter((item) => Date.parse(item.responseDeadlineAt) - Date.now() <= 1000 * 60 * 60 * 72).length;
  const awarded = data.activeOrders.slice(0, 3);
  const awardedWorkHref = awarded[0] ? `/buyer/orders/${awarded[0].id}` : "/buyer";

  return (
    <WorkspaceShell
      role="buyer"
      title="Open request board"
      description="See the market the same way providers do: budget, low proposal, proposal count, and deadline in one place."
      status={`${openRequests.length} live requests`}
      actions={[
        { href: "/buyer/rfqs/create", label: "Post request" },
        { href: awardedWorkHref, label: "Awarded work", variant: "outline" },
      ]}
    >
      <section className="rounded-md border border-border bg-card">
        <div className="grid gap-px bg-border lg:grid-cols-[1.15fr_0.95fr_0.9fr_0.9fr]">
          <MetricStrip
            icon={RiAuctionLine}
            label="Live request book"
            value={openRequests.length ? `${openRequests.length} requests` : "No live requests"}
            detail="Price and timing stay visible."
          />
          <MetricStrip
            icon={RiPriceTag3Line}
            label="Highest posted budget"
            value={topRequest ? formatMoney(topRequest.budgetCents) : "-"}
            detail={topRequest?.title ?? "No open request"}
          />
          <MetricStrip
            icon={RiAuctionLine}
            label="Lowest live price"
            value={liveFloor ? formatMoney(liveFloor) : "-"}
            detail={liveFloor ? "Current market floor" : "No price yet"}
          />
          <MetricStrip
            icon={RiTimeLine}
            label="Closing in 72 hrs"
            value={`${closingSoon}`}
            detail="Requests nearing award"
          />
        </div>
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.62fr_0.82fr] xl:items-start">
        <SectionCard
          eyebrow="Live request feed"
          title="Price-first request feed"
          description="This board stays focused on the numbers that decide who gets the work."
        >
          <div className="space-y-3">
            {openRequests.length === 0 ? (
              <Card className="border-dashed p-6 text-sm text-muted-foreground">No live requests right now. Post a new request to start a market.</Card>
            ) : (
              openRequests.slice(0, 8).map((item, index) => {
                const low = getLowestBid(item);

                return (
                  <div key={item.id} className="market-row">
                    <div className="grid gap-4 xl:grid-cols-[1.7fr_0.72fr_0.72fr_0.62fr_0.62fr_auto] xl:items-center">
                      <div className="min-w-0 space-y-2">
                        <div className="text-xs font-medium text-primary">{marketSignal(item.bidCount, index)}</div>
                        <h3 className="text-lg font-semibold leading-tight tracking-tight break-words text-balance">{item.title}</h3>
                      </div>
                      <FeedMetric label="Budget" value={formatMoney(item.budgetCents)} emphasize />
                      <FeedMetric label="Current low" value={low ? formatMoney(low.quoteCents) : "No price"} />
                      <FeedMetric label="Spread" value={low ? formatMoney(Math.max(item.budgetCents - low.quoteCents, 0)) : "Open"} />
                      <div className="space-y-1 text-sm text-muted-foreground">
                        <div>{formatProposalCount(item.bidCount)}</div>
                        <div>{formatDate(item.responseDeadlineAt)}</div>
                      </div>
                      {low ? (
                        <form action={`/buyer/rfqs/${item.id}/award`} method="post">
                          <input type="hidden" name="bidId" value={low.id} />
                          <input type="hidden" name="fundingMode" value="credit" />
                          <input type="hidden" name="creditLineId" value="credit_1" />
                          <Button type="submit" className="w-full xl:w-auto">
                            Award low
                            <RiArrowRightUpLine className="size-4" />
                          </Button>
                        </form>
                      ) : (
                        <Button disabled variant="outline" className="w-full xl:w-auto">
                          Waiting
                        </Button>
                      )}
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </SectionCard>

        <div className="space-y-6">
          <SectionCard
            eyebrow="Top request now"
            title={topRequest?.title ?? "No live request"}
            description="This is the clearest current market view for a client watching proposals come in."
          >
            {topRequest ? (
              <div className="space-y-4">
                <div className="grid gap-3 sm:grid-cols-2">
                  <SpotMetric label="Budget" value={formatMoney(topRequest.budgetCents)} />
                  <SpotMetric label="Current low" value={getLowestBid(topRequest) ? formatMoney(getLowestBid(topRequest)!.quoteCents) : "No price"} />
                  <SpotMetric label="Proposal count" value={`${topRequest.bidCount}`} />
                  <SpotMetric label="Deadline" value={formatDate(topRequest.responseDeadlineAt)} />
                </div>
                {getLowestBid(topRequest) ? (
                  <form action={`/buyer/rfqs/${topRequest.id}/award`} method="post">
                    <input type="hidden" name="bidId" value={getLowestBid(topRequest)!.id} />
                    <input type="hidden" name="fundingMode" value="credit" />
                    <input type="hidden" name="creditLineId" value="credit_1" />
                    <Button type="submit" className="w-full justify-between">
                      Award lowest proposal
                      <RiArrowRightUpLine className="size-4" />
                    </Button>
                  </form>
                ) : (
                  <Button asChild className="w-full justify-between">
                    <Link href="/buyer/rfqs/create">
                      Post another request
                      <RiArrowRightUpLine className="size-4" />
                    </Link>
                  </Button>
                )}
              </div>
            ) : (
              <Card className="border-dashed p-6 text-sm text-muted-foreground">No live request is available right now.</Card>
            )}
          </SectionCard>

          <SectionCard
            eyebrow="Awarded work"
            title="Closed on price"
            description="These requests already moved out of market pricing and into delivery."
          >
            <div className="grid gap-4">
              {awarded.length === 0 ? (
                <Card className="border-dashed p-6 text-sm text-muted-foreground">No awarded requests yet.</Card>
              ) : (
                awarded.map((order) => (
                  <div key={order.id} className="market-card p-5">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 space-y-2">
                        <div className="text-xs text-primary">{order.status === "running" ? "In delivery" : order.status}</div>
                        <h3 className="text-lg font-semibold tracking-tight break-words text-balance">{order.id}</h3>
                      </div>
                      <div className="font-mono text-xl font-semibold tabular-nums text-foreground">{order.milestones.length}</div>
                    </div>
                    <div className="mt-4 text-sm text-muted-foreground">Provider {order.providerOrgId}</div>
                    <Button asChild variant="outline" className="mt-5 w-full justify-between">
                      <Link href={`/buyer/orders/${order.id}`}>
                        View delivery
                        <RiArrowRightUpLine className="size-4" />
                      </Link>
                    </Button>
                  </div>
                ))
              )}
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
  icon: typeof RiAuctionLine;
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

function FeedMetric({
  label,
  value,
  emphasize = false,
}: {
  label: string;
  value: string;
  emphasize?: boolean;
}) {
  return (
    <div className="min-w-0">
      <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{label}</div>
      <div className={emphasize ? "price-inline mt-2 break-words" : "mt-2 font-mono text-xl font-semibold leading-tight tracking-tight tabular-nums break-words text-foreground"}>
        {value}
      </div>
    </div>
  );
}

function SpotMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-border bg-secondary/50 px-4 py-4">
      <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{label}</div>
      <div className="mt-2 font-mono text-2xl font-semibold tracking-tight tabular-nums text-foreground">{value}</div>
    </div>
  );
}

function getLowestBid(item: { bids: Array<{ id: string; quoteCents: number }> }) {
  if (item.bids.length === 0) return null;
  return [...item.bids].sort((left, right) => left.quoteCents - right.quoteCents)[0] ?? null;
}

function marketSignal(bidCount: number, index: number) {
  if (index === 0) return "Top budget now";
  if (bidCount >= 6) return "Crowded pricing";
  if (bidCount === 0) return "Waiting on first price";
  return "Open for award";
}

function formatProposalCount(count: number) {
  return count === 1 ? "1 live proposal" : `${count} live proposals`;
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric" }).format(new Date(value));
}
