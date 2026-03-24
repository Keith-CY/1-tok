import Link from "next/link";
import { RiArrowRightUpLine, RiAuctionLine, RiFlashlightLine, RiPriceTag3Line, RiSearchLine, RiTimeLine } from "react-icons/ri";

import { formatMoney } from "@1tok/contracts";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { getProviderDashboardData } from "@/lib/api";
import { requirePortalViewer } from "@/lib/viewer";

export const metadata = {
  title: "Provider",
};

export const dynamic = "force-dynamic";

const errorMessages: Record<string, string> = {
  "bid-invalid": "Enter a price before submitting.",
  "bid-submit-failed": "Proposal submission failed. Try again.",
};

export default async function ProviderPage({
  searchParams,
}: {
  searchParams?: Promise<{ error?: string }>;
}) {
  const viewer = await requirePortalViewer("provider", "/provider");
  const data = await getProviderDashboardData({
    authToken: viewer.token,
    providerOrgId: viewer.membership.organization.id,
    requireLive: true,
  });
  const params = await searchParams;
  const error = params?.error ? errorMessages[params.error] ?? "Something went wrong. Try again." : null;

  const available = [...data.marketOpportunities]
    .filter((item) => !item.hasProviderBid)
    .sort(
      (left, right) =>
        right.budgetCents - left.budgetCents ||
        (left.lowestQuoteCents ?? Number.MAX_SAFE_INTEGER) - (right.lowestQuoteCents ?? Number.MAX_SAFE_INTEGER) ||
        Date.parse(left.responseDeadlineAt) - Date.parse(right.responseDeadlineAt),
    );
  const submitted = data.marketQueue.filter((item) => item.providerBidStatus !== "awarded" && item.providerBidStatus !== "rejected").slice(0, 4);
  const active = data.marketQueue.filter((item) => item.providerBidStatus === "awarded").slice(0, 3);
  const topBudget = available[0]?.budgetCents ?? null;
  const liveFloor = available
    .map((item) => item.lowestQuoteCents)
    .filter((value): value is number => value !== null)
    .sort((left, right) => left - right)[0] ?? null;
  const closingSoon = available.filter((item) => Date.parse(item.responseDeadlineAt) - Date.now() <= 1000 * 60 * 60 * 72).length;
  const topRequest = available[0] ?? null;

  return (
    <WorkspaceShell
      role="provider"
      title="Price the open board"
      description="Scan budgets, quote into live requests, and keep comparisons and awarded work in one provider lane."
      status={`${available.length} live requests`}
      actions={[
        { href: "/provider/rfqs", label: "Open requests" },
        { href: "/provider/proposals", label: "My proposals", variant: "outline" },
      ]}
    >
      {error ? <div className="rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</div> : null}

      <section className="sheet-stack overflow-hidden">
        <div className="grid gap-px bg-border lg:grid-cols-[1.15fr_0.95fr_0.9fr_0.9fr]">
          <MetricStrip
            icon={RiFlashlightLine}
            label="Live market read"
            value={available.length ? `${available.length} requests` : "No live requests"}
            detail="Sorted for fast pricing decisions."
          />
          <MetricStrip
            icon={RiPriceTag3Line}
            label="Highest open budget"
            value={topBudget ? formatMoney(topBudget) : "-"}
            detail={topRequest?.title ?? "No open request"}
          />
          <MetricStrip
            icon={RiAuctionLine}
            label="Lowest live proposal"
            value={liveFloor ? formatMoney(liveFloor) : "-"}
            detail={liveFloor ? "Current market floor" : "No live floor yet"}
          />
          <MetricStrip
            icon={RiTimeLine}
            label="Closing in 72 hrs"
            value={`${closingSoon}`}
            detail="Requests nearing decision"
          />
        </div>
      </section>

      <section className="mt-6 rounded-md bg-[var(--surface-lowest)] px-6 py-5 shadow-[0_20px_40px_rgba(0,0,0,0.06)] sm:px-7">
        <div className="space-y-2">
          <div className="inline-flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.22em] text-accent">
            <RiSearchLine className="size-4" />
            Demo market note
          </div>
          <p className="text-sm leading-7 text-foreground">
            This demo board clears fastest when buyers post short research requests. Typical demo budgets land in the $300-$800 range, and early quotes usually have the clearest edge.
          </p>
        </div>
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.62fr_0.82fr] xl:items-start">
        <SectionCard
          eyebrow="Open board"
          title="Requests worth pricing now"
          description="Keep budget, current low, proposal pressure, and deadline in the same line of sight."
        >
          <div className="space-y-3">
            {available.length === 0 ? (
              <Card className="market-card p-6">
                <div className="space-y-5">
                  <div className="space-y-3">
                    <div className="eyebrow-pill">No live request</div>
                    <h3 className="font-display text-[clamp(1.8rem,2.8vw,2.4rem)] font-medium leading-[1.02] tracking-[-0.03em] text-balance">
                      The board is quiet right now.
                    </h3>
                    <p className="max-w-2xl text-sm leading-7 text-muted-foreground">
                      When a buyer posts a fresh research request, this feed turns back into the price-first market view. Until then, keep an eye on short analytic tasks with clear budgets and fast award windows.
                    </p>
                  </div>

                  <div className="grid gap-px bg-border sm:grid-cols-2">
                    <GuideMetric label="Typical budget" value="$300-$800" detail="most demo requests" />
                    <GuideMetric label="Award pace" value="2.4 hr" detail="common time to decision" />
                  </div>
                </div>
              </Card>
            ) : (
              available.slice(0, 8).map((item, index) => (
                <div key={item.id} className="market-row">
                  <div className="grid gap-4 xl:grid-cols-[1.7fr_0.72fr_0.72fr_0.62fr_0.62fr_auto] xl:items-center">
                    <div className="min-w-0 space-y-2">
                      <div className="text-xs font-medium text-primary">{marketSignal(item.budgetCents, item.proposalCount, index)}</div>
                      <h3 className="text-lg font-semibold leading-tight tracking-tight break-words text-balance">{item.title}</h3>
                    </div>
                    <FeedMetric label="Budget" value={formatMoney(item.budgetCents)} emphasize />
                    <FeedMetric label="Current low" value={item.lowestQuoteCents ? formatMoney(item.lowestQuoteCents) : "Be first"} />
                    <FeedMetric label="Spread" value={spread(item.budgetCents, item.lowestQuoteCents)} />
                    <div className="space-y-1 text-sm text-muted-foreground">
                      <div>{formatProposalCount(item.proposalCount)}</div>
                      <div>{formatDate(item.responseDeadlineAt)}</div>
                    </div>
                    <Button asChild className="w-full xl:w-auto">
                      <Link href={`/provider/rfqs/${item.id}`}>
                        Open request
                        <RiArrowRightUpLine className="size-4" />
                      </Link>
                    </Button>
                  </div>
                </div>
              ))
            )}
          </div>
        </SectionCard>

        <div className="space-y-6">
          <SectionCard
            eyebrow={topRequest ? "Top request now" : "Demo market read"}
            title={topRequest?.title ?? "When a fresh request lands"}
            description={
              topRequest
                ? "This is the clearest current entry point for a provider scanning the market."
                : "Use this as the provider-side calibration for the demo market before the next request opens."
            }
          >
            {topRequest ? (
              <div className="space-y-4">
                <div className="grid gap-3 sm:grid-cols-2">
                  <SpotMetric label="Budget" value={formatMoney(topRequest.budgetCents)} />
                  <SpotMetric
                    label="Current low"
                    value={topRequest.lowestQuoteCents ? formatMoney(topRequest.lowestQuoteCents) : "Be first"}
                  />
                  <SpotMetric label="Proposal count" value={`${topRequest.proposalCount}`} />
                  <SpotMetric label="Deadline" value={formatDate(topRequest.responseDeadlineAt)} />
                </div>
                <Button asChild className="w-full justify-between">
                  <Link href={`/provider/rfqs/${topRequest.id}`}>
                    Open top request
                    <RiArrowRightUpLine className="size-4" />
                  </Link>
                </Button>
              </div>
            ) : (
              <div className="space-y-5">
                <div className="grid gap-3 sm:grid-cols-2">
                  <SpotMetric label="Task shape" value="Research" />
                  <SpotMetric label="Typical budget" value="$300-$800" />
                  <SpotMetric label="Award window" value="~2 hr" />
                  <SpotMetric label="Best opening" value="Quote early" />
                </div>
                <Button asChild className="w-full justify-between">
                  <Link href="/provider/rfqs">
                    Open request feed
                    <RiArrowRightUpLine className="size-4" />
                  </Link>
                </Button>
              </div>
            )}
          </SectionCard>

          <SectionCard eyebrow="My proposals" title="Already in comparison" description="Prices the client is already weighing against other providers.">
            <div className="grid gap-4">
              {submitted.length === 0 ? (
                <Card className="market-card p-5">
                  <div className="space-y-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-accent">No live proposal</div>
                    <p className="text-sm leading-7 text-muted-foreground">
                      Once you quote, live comparisons stay here until the client awards or rejects the request.
                    </p>
                    <Button asChild variant="outline" className="w-full justify-between">
                      <Link href="/provider/rfqs">
                        Browse open requests
                        <RiArrowRightUpLine className="size-4" />
                      </Link>
                    </Button>
                  </div>
                </Card>
              ) : (
                submitted.map((item) => (
                  <CompactProposalCard
                    key={item.id}
                    title={item.title}
                    price={formatMoney(item.quoteCents)}
                    deadline={formatDate(item.responseDeadlineAt)}
                    status="Live proposal"
                    href={`/provider/rfqs/${item.rfqId}`}
                  />
                ))
              )}
            </div>
          </SectionCard>

          <SectionCard eyebrow="Awarded work" title="Closed on price" description="These requests already moved out of market pricing and into delivery.">
            <div className="grid gap-4">
              {active.length === 0 ? (
                <Card className="market-card p-5">
                  <div className="space-y-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-accent">No awarded work</div>
                    <p className="text-sm leading-7 text-muted-foreground">
                      Orders appear here after a client accepts your price and the request leaves the market board.
                    </p>
                    <Button asChild variant="outline" className="w-full justify-between">
                      <Link href="/provider/proposals">
                        Review my proposals
                        <RiArrowRightUpLine className="size-4" />
                      </Link>
                    </Button>
                  </div>
                </Card>
              ) : (
                active.map((item) => (
                  <CompactProposalCard
                    key={item.id}
                    title={item.title}
                    price={formatMoney(item.quoteCents)}
                    deadline={formatDate(item.responseDeadlineAt)}
                    status="Awarded"
                    href={item.orderId ? `/provider/orders/${item.orderId}` : `/provider/rfqs/${item.rfqId}`}
                  />
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
  icon: typeof RiFlashlightLine;
  label: string;
  value: string;
  detail: string;
}) {
  return (
    <div className="flex min-h-[11.5rem] flex-col justify-between bg-[var(--surface-lowest)] px-5 py-6">
      <div className="space-y-3">
        <div className="inline-flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
          <Icon className="size-4 text-accent" />
          {label}
        </div>
        <div className="max-w-[13rem] font-display text-[2rem] font-medium leading-[1.02] tracking-[-0.03em] text-foreground">
          {value}
        </div>
      </div>
      <div className="max-w-[15rem] text-sm leading-7 text-muted-foreground">{detail}</div>
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
      <div className={emphasize ? "price-inline mt-2 break-words" : "mt-2 font-mono text-xl font-semibold leading-tight tracking-tight tabular-nums break-words text-foreground"}>{value}</div>
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

function GuideMetric({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="bg-[var(--surface-lowest)] px-5 py-5">
      <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">{label}</div>
      <div className="mt-3 font-mono text-2xl font-semibold tracking-tight tabular-nums text-foreground">{value}</div>
      <div className="mt-3 text-sm leading-7 text-muted-foreground">{detail}</div>
    </div>
  );
}

function CompactProposalCard({
  title,
  price,
  deadline,
  status,
  href,
}: {
  title: string;
  price: string;
  deadline: string;
  status: string;
  href: string;
}) {
  return (
    <div className="market-card p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="space-y-2">
          <div className="text-xs text-primary">{status}</div>
          <h3 className="text-lg font-semibold tracking-tight text-balance">{title}</h3>
        </div>
        <div className="font-mono text-xl font-semibold tabular-nums text-foreground">{price}</div>
      </div>
      <div className="mt-4 text-sm text-muted-foreground">Delivery window {deadline}</div>
      <Button asChild variant="outline" className="mt-5 w-full justify-between">
        <Link href={href}>
          View request
          <RiArrowRightUpLine className="size-4" />
        </Link>
      </Button>
    </div>
  );
}

function marketSignal(budgetCents: number, proposalCount: number, index: number) {
  if (index === 0) return "Top budget now";
  if (proposalCount >= 6) return "Crowded pricing";
  if (budgetCents >= 500000) return "High budget";
  if (proposalCount === 0) return "First proposal wins";
  return "Open for pricing";
}

function formatProposalCount(count: number) {
  return count === 1 ? "1 live proposal" : `${count} live proposals`;
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric" }).format(new Date(value));
}

function spread(budgetCents: number, lowestQuoteCents: number | null) {
  if (!lowestQuoteCents) return "Open";
  return formatMoney(Math.max(budgetCents - lowestQuoteCents, 0));
}
