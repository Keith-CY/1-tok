import Link from "next/link";
import { RiArrowRightUpLine } from "react-icons/ri";

import { formatMoney } from "@1tok/contracts";
import { BuyerDepositPanel } from "@/components/buyer-deposit-panel";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { getBuyerDashboardData } from "@/lib/api";
import { requirePortalViewer } from "@/lib/viewer";

export const metadata = {
  title: "Buyer",
};

export const dynamic = "force-dynamic";

export default async function BuyerPage({
  searchParams,
}: {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
}) {
  const viewer = await requirePortalViewer("buyer", "/buyer");
  const data = await getBuyerDashboardData({
    authToken: viewer.token,
    buyerOrgId: viewer.membership.organization.id,
    requireLive: true,
  });
  const params = await searchParams;
  const topUp = String(params?.topup ?? "");
  const deposit = data.deposit;

  const openRequests = [...data.rfqBook]
    .filter((item) => item.status === "open")
    .sort((left, right) => Date.parse(right.createdAt) - Date.parse(left.createdAt));
  const topRequest = openRequests[0] ?? null;
  const liveFloor = openRequests
    .flatMap((item) => item.bids.map((bid) => bid.quoteCents))
    .sort((left, right) => left - right)[0] ?? null;
  const closingSoon = openRequests.filter((item) => Date.parse(item.responseDeadlineAt) - Date.now() <= 1000 * 60 * 60 * 72).length;
  const awarded = data.activeOrders.slice(0, 3);

  return (
    <WorkspaceShell
      role="buyer"
      title="Request board"
      description="Post against a visible budget, watch proposals clear into view, and move winning work into delivery."
      status={`${openRequests.length} open requests`}
    >
      <section className="grid gap-3 lg:grid-cols-[1.15fr_0.95fr_0.9fr_0.9fr]">
        <MetricStrip
          label="Live request book"
          value={openRequests.length ? `${openRequests.length} requests` : "No live requests"}
          detail="Price and timing stay visible."
        />
        <MetricStrip
          label="Prepaid balance"
          value={formatMoney(data.summary.prepaidBalanceCents)}
          detail={deposit ? `${deposit.creditedBalance} credited from CKB deposits` : `${data.summary.settledTopUps} settled USDI top-ups`}
        />
        <MetricStrip
          label="Lowest live price"
          value={liveFloor ? formatMoney(liveFloor) : "-"}
          detail={liveFloor ? "Current market floor" : "No price yet"}
        />
        <MetricStrip
          label="Closing in 72 hrs"
          value={`${closingSoon}`}
          detail="Requests nearing award"
        />
      </section>

      <section className="grid gap-8 xl:grid-cols-[minmax(0,1.56fr)_22rem] xl:items-start">
        <SectionCard
          eyebrow="Open requests"
          title="Price-first request feed"
          description="Every row holds the same facts providers see before they quote: budget, current floor, spread, and deadline."
        >
          <div className="space-y-3">
            {openRequests.length === 0 ? (
              <Card className="bg-card p-6 text-sm text-muted-foreground">No live requests right now. Post a new request to start a market.</Card>
            ) : (
              openRequests.slice(0, 8).map((item, index) => {
                const low = getLowestBid(item);

                return (
                  <div key={item.id} className="market-row">
                    <div className="grid gap-5 xl:grid-cols-[1.7fr_0.74fr_0.74fr_0.68fr_0.62fr_auto] xl:items-end">
                      <div className="min-w-0 space-y-3">
                        <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-accent">{marketSignal(item.bidCount, index)}</div>
                        <h3 className="font-display text-2xl font-medium leading-[1.05] tracking-[-0.02em] break-words text-balance">{item.title}</h3>
                      </div>
                      <FeedMetric label="Budget" value={formatMoney(item.budgetCents)} emphasize />
                      <FeedMetric label="Current low" value={low ? formatMoney(low.quoteCents) : "No price"} />
                      <FeedMetric label="Spread" value={low ? formatMoney(Math.max(item.budgetCents - low.quoteCents, 0)) : "Open"} />
                      <div className="space-y-2 text-sm leading-7 text-muted-foreground">
                        <div>{formatProposalCount(item.bidCount)}</div>
                        <div>{formatDate(item.responseDeadlineAt)}</div>
                      </div>
                      {low ? (
                        <form action={`/buyer/rfqs/${item.id}/award`} method="post">
                          <input type="hidden" name="bidId" value={low.id} />
                          <input type="hidden" name="fundingMode" value="prepaid" />
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

        <div className="space-y-8">
          <SectionCard
            eyebrow="Client controls"
            title="Post and review"
            description="Keep new work, funded balance, and delivery follow-up in one compact rail."
          >
            <div className="grid gap-3">
              <Button asChild className="w-full justify-between">
                <Link href="/buyer/rfqs/create">
                  Post request
                  <RiArrowRightUpLine className="size-4" />
                </Link>
              </Button>
              <Button asChild variant="outline" className="w-full justify-between">
                <Link href="/buyer/delivery">
                  Review delivery
                  <RiArrowRightUpLine className="size-4" />
                </Link>
              </Button>
            </div>
          </SectionCard>

          <SectionCard eyebrow="Fund buyer balance" title="">
            <div className="space-y-4">
              <BuyerDepositPanel deposit={deposit} topUp={topUp} />

              <div className="market-card px-5 py-5 opacity-60">
                <div className="space-y-3">
                  <div className="flex items-start justify-between gap-3">
                    <div className="space-y-2">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">Bank top up USD</div>
                      <div className="font-display text-[clamp(1.15rem,1.7vw,1.45rem)] font-medium leading-[1] tracking-[-0.03em] text-balance">
                        USD bank funding
                      </div>
                    </div>
                    <span className="bg-card px-3 py-2 text-[10px] font-semibold uppercase tracking-[0.22em] text-muted-foreground">
                      Coming soon
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </SectionCard>

          <SectionCard
            eyebrow="Top request"
            title={topRequest?.title ?? "No live request"}
            description="The clearest current market view for the client side of the board."
          >
            {topRequest ? (
              <div className="space-y-5">
                <div className="grid gap-3 sm:grid-cols-2">
                  <SpotMetric label="Budget" value={formatMoney(topRequest.budgetCents)} />
                  <SpotMetric label="Current low" value={getLowestBid(topRequest) ? formatMoney(getLowestBid(topRequest)!.quoteCents) : "No price"} />
                  <SpotMetric label="Proposal count" value={`${topRequest.bidCount}`} />
                  <SpotMetric label="Deadline" value={formatDate(topRequest.responseDeadlineAt)} />
                </div>
                {getLowestBid(topRequest) ? (
                  <form action={`/buyer/rfqs/${topRequest.id}/award`} method="post">
                    <input type="hidden" name="bidId" value={getLowestBid(topRequest)!.id} />
                    <input type="hidden" name="fundingMode" value="prepaid" />
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
              <Card className="bg-card p-6 text-sm text-muted-foreground">No live request is available right now.</Card>
            )}
          </SectionCard>

          <SectionCard
            eyebrow="Delivery lane"
            title="Closed on price"
            description="Requests that already cleared the board and moved into execution."
          >
            <div className="grid gap-4">
              {awarded.length === 0 ? (
                <Card className="bg-card p-6 text-sm text-muted-foreground">No awarded requests yet.</Card>
              ) : (
                awarded.map((order) => (
                  <div key={order.id} className="market-card p-5">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 space-y-2">
                        <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-accent">
                          {order.status === "running" ? "In delivery" : order.status}
                        </div>
                        <h3 className="font-display text-2xl font-medium tracking-[-0.02em] break-words text-balance">{order.id}</h3>
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
  label,
  value,
  detail,
}: {
  label: string;
  value: string;
  detail: string;
}) {
  return (
    <div className="market-card px-5 py-6">
      <div className="space-y-3">
        <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">{label}</div>
        <div className="font-mono text-3xl font-semibold tracking-tight tabular-nums text-foreground">{value}</div>
        <div className="max-w-xs text-sm leading-7 text-muted-foreground">{detail}</div>
      </div>
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
      <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-muted-foreground">{label}</div>
      <div className={emphasize ? "price-inline mt-3 break-words" : "mt-3 font-mono text-xl font-semibold leading-tight tracking-tight tabular-nums break-words text-foreground"}>
        {value}
      </div>
    </div>
  );
}

function SpotMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-card px-5 py-5">
      <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">{label}</div>
      <div className="mt-3 font-mono text-2xl font-semibold tracking-tight tabular-nums text-foreground">{value}</div>
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
