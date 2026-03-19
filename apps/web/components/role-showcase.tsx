import { RiAuctionLine, RiFlashlightLine, RiPulseLine, RiTimeLine } from "react-icons/ri";

const featuredRequest = {
  title: "Carrier dispute response package",
  budget: "$8.4k",
  currentLow: "$6.8k",
  proposals: 12,
  deadline: "36 hrs",
};

const requestRows = [
  { title: "Carrier onboarding pack", budget: "$4.2k", currentLow: "$3.9k", proposals: 8, deadline: "5 days" },
  { title: "Settlement reconciliation sprint", budget: "$5.1k", currentLow: "$4.6k", proposals: 6, deadline: "72 hrs" },
  { title: "Milestone delivery audit", budget: "$6.1k", currentLow: "$5.2k", proposals: 5, deadline: "4 days" },
] as const;

const latestProposals = [
  { provider: "North Studio", request: "Incident review", price: "$6.8k" },
  { provider: "Kite Works", request: "Onboarding pack", price: "$3.9k" },
  { provider: "Mono Labs", request: "Delivery audit", price: "$5.2k" },
] as const;

export function RoleShowcase({ compact = false }: { compact?: boolean }) {
  const rows = compact ? requestRows.slice(0, 2) : requestRows;
  const proposals = compact ? latestProposals.slice(0, 2) : latestProposals;

  return (
    <section className="market-card overflow-hidden">
      <div className="border-b border-border/70 px-6 py-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="space-y-3">
            <div className="eyebrow-pill">
              <RiFlashlightLine className="size-3.5" />
              Live request board
            </div>
            <div className="space-y-3">
              <h2 className="max-w-2xl font-display text-4xl leading-[0.98] tracking-tight text-balance text-foreground">
                Budgets are public. Providers are already writing against them.
              </h2>
              <p className="max-w-2xl text-sm leading-7 text-muted-foreground">
                The market reads like a broadsheet: budget, low proposal, proposal count, and timing remain visible without dashboard noise.
              </p>
            </div>
          </div>
          <div className="rounded-full bg-[var(--ink-accent-weak)] px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.22em] text-primary">
            Pricing live now
          </div>
        </div>
      </div>

      <div className="grid gap-8 px-6 py-6 xl:grid-cols-[1.2fr_0.8fr]">
        <div className="space-y-6">
          <div className="rounded-[1.25rem] border border-border/70 bg-secondary/70 px-5 py-5">
            <div className="grid gap-5 lg:grid-cols-[1.2fr_0.8fr] lg:items-end">
              <div className="min-w-0 space-y-3">
                <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-primary">Highest live budget</div>
                <div className="max-w-[16ch] font-display text-[2.6rem] leading-[0.95] tracking-tight text-balance text-foreground">
                  {featuredRequest.title}
                </div>
                <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
                  <span>{featuredRequest.proposals} proposals</span>
                  <span>{featuredRequest.deadline}</span>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <MetricTile label="Budget" value={featuredRequest.budget} className="col-span-2" />
                <MetricTile label="Low" value={featuredRequest.currentLow} icon={RiPulseLine} />
                <MetricTile label="Spread" value={spread(featuredRequest.budget, featuredRequest.currentLow)} icon={RiAuctionLine} />
              </div>
            </div>
          </div>

          <div className="space-y-1">
            <div className="grid grid-cols-[1.7fr_0.72fr_0.72fr_0.78fr] gap-4 px-1 pb-3 text-[11px] font-semibold uppercase tracking-[0.22em] text-muted-foreground">
              <div>Request</div>
              <div>Budget</div>
              <div>Low proposal</div>
              <div>Activity</div>
            </div>
            <div className="space-y-0">
              {rows.map((item) => (
                <div key={item.title} className="market-row grid gap-4 md:grid-cols-[1.7fr_0.72fr_0.72fr_0.78fr] md:items-center">
                  <div className="min-w-0 space-y-1">
                    <div className="text-base font-semibold leading-tight text-foreground">{item.title}</div>
                    <div className="text-xs text-muted-foreground">Live pricing on 1-tok</div>
                  </div>
                  <ValueBlock value={item.budget} />
                  <ValueBlock value={item.currentLow} />
                  <div className="space-y-1 text-sm text-muted-foreground">
                    <div>{item.proposals} proposals</div>
                    <div className="inline-flex items-center gap-1">
                      <RiTimeLine className="size-4 text-primary" />
                      {item.deadline}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="rounded-[1.25rem] border border-border/70 bg-card/86 px-5 py-5">
          <div className="space-y-1">
            <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-primary">Latest proposals</div>
            <div className="font-display text-3xl leading-none tracking-tight text-foreground">Operational mode</div>
            <div className="text-sm leading-7 text-muted-foreground">Dense enough for decisioning, spare enough to stay readable.</div>
          </div>

          <div className="mt-6 space-y-0">
            {proposals.map((proposal) => (
              <div
                key={`${proposal.provider}-${proposal.request}`}
                className="flex items-center justify-between gap-3 border-t border-border/70 py-4 first:border-t-0 first:pt-0"
              >
                <div className="space-y-1">
                  <div className="text-sm font-semibold text-foreground">{proposal.provider}</div>
                  <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{proposal.request}</div>
                </div>
                <div className="font-mono text-xl font-semibold tracking-tight tabular-nums text-foreground">{proposal.price}</div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function MetricTile({
  label,
  value,
  icon: Icon,
  className,
}: {
  label: string;
  value: string;
  icon?: typeof RiPulseLine;
  className?: string;
}) {
  return (
    <div className={`rounded-[1rem] border border-border/70 bg-card/90 px-4 py-4 ${className ?? ""}`}>
      <div className="inline-flex items-center gap-1 text-[11px] font-semibold uppercase tracking-[0.2em] text-muted-foreground">
        {Icon ? <Icon className="size-4 text-primary" /> : null}
        {label}
      </div>
      <div className="mt-3 min-w-0 font-mono text-[1.65rem] font-semibold leading-none tracking-tight tabular-nums text-foreground">{value}</div>
    </div>
  );
}

function ValueBlock({ value }: { value: string }) {
  return <div className="font-mono text-xl font-semibold leading-none tracking-tight tabular-nums text-foreground">{value}</div>;
}

function spread(budget: string, currentLow: string) {
  const budgetValue = Number.parseFloat(budget.replace(/[$k,]/g, ""));
  const lowValue = Number.parseFloat(currentLow.replace(/[$k,]/g, ""));
  const multiplier = budget.includes("k") ? 1000 : 1;
  const budgetAmount = budgetValue * multiplier;
  const lowAmount = lowValue * multiplier;
  const delta = Math.max(budgetAmount - lowAmount, 0);

  if (delta >= 1000) {
    return `$${(delta / 1000).toFixed(1)}k`;
  }

  return `$${delta.toFixed(0)}`;
}
