import { RiAuctionLine, RiPulseLine, RiTimeLine } from "react-icons/ri";

const featuredRequest = {
  title: "Carrier dispute response package",
  budget: "$780",
  currentLow: "$620",
  proposals: 12,
  deadline: "2 hrs",
};

const requestRows = [
  { title: "Carrier onboarding pack", budget: "$420", currentLow: "$360", proposals: 8, deadline: "2 hrs" },
  { title: "Settlement reconciliation sprint", budget: "$560", currentLow: "$480", proposals: 6, deadline: "2 hrs" },
  { title: "Milestone delivery audit", budget: "$720", currentLow: "$640", proposals: 5, deadline: "2 hrs" },
] as const;

const latestProposals = [
  { provider: "North Studio", request: "Incident review", price: "$620" },
  { provider: "Kite Works", request: "Onboarding pack", price: "$360" },
  { provider: "Mono Labs", request: "Delivery audit", price: "$640" },
] as const;

export function RoleShowcase({ compact = false }: { compact?: boolean }) {
  const rows = compact ? requestRows.slice(0, 2) : requestRows;
  const proposals = compact ? latestProposals.slice(0, 2) : latestProposals;

  return (
    <section className="bg-card shadow-[0_20px_40px_rgba(0,0,0,0.06)]">
      <div className="bg-secondary px-6 py-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="space-y-2">
            <div className="eyebrow-pill">Request board</div>
            <div className="space-y-2">
              <h2 className="max-w-2xl font-display text-[clamp(2.2rem,3.2vw,3.4rem)] font-medium leading-[1.02] tracking-[-0.03em] text-balance">
                Budgets stay visible. Providers price against the same sheet.
              </h2>
              <p className="text-sm leading-7 text-muted-foreground">
                The board stays readable because 1-tok keeps four signals together: budget, low proposal, proposal count, and delivery timing.
              </p>
            </div>
          </div>
          <div className="bg-card px-3 py-2 text-xs font-medium text-foreground shadow-[0_20px_40px_rgba(0,0,0,0.06)]">
            Price in view
          </div>
        </div>
      </div>

      <div className="space-y-6 px-6 py-6">
        <div className="bg-secondary p-5">
          <div className="grid gap-5 lg:grid-cols-[1.25fr_0.75fr] lg:items-end">
            <div className="min-w-0 space-y-3">
              <div className="text-xs font-medium text-primary">Highest budget</div>
              <div className="max-w-[16ch] font-display text-[2.4rem] font-medium leading-[1.02] tracking-[-0.03em] text-balance">{featuredRequest.title}</div>
              <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                <span className="bg-card px-2 py-1 shadow-[0_20px_40px_rgba(0,0,0,0.06)]">{featuredRequest.proposals} proposals</span>
                <span className="bg-card px-2 py-1 shadow-[0_20px_40px_rgba(0,0,0,0.06)]">{featuredRequest.deadline}</span>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <MetricTile label="Budget" value={featuredRequest.budget} className="col-span-2" />
              <MetricTile label="Low" value={featuredRequest.currentLow} icon={RiPulseLine} />
              <MetricTile label="Spread" value={spread(featuredRequest.budget, featuredRequest.currentLow)} icon={RiAuctionLine} />
            </div>
          </div>
        </div>

        <div className="overflow-hidden bg-secondary">
          <div className="grid gap-px bg-background/50">
            <div className="hidden grid-cols-[1.7fr_0.7fr_0.7fr_0.7fr_0.72fr] bg-secondary px-4 py-3 text-[11px] uppercase tracking-[0.16em] text-muted-foreground md:grid">
              <div>Request</div>
              <div>Budget</div>
              <div>Low proposal</div>
              <div>Spread</div>
              <div>Activity</div>
            </div>
            {rows.map((item) => (
              <div
                key={item.title}
                className="grid gap-4 bg-card px-4 py-4 transition-colors duration-150 hover:bg-secondary md:grid-cols-[1.7fr_0.7fr_0.7fr_0.7fr_0.72fr] md:items-center"
              >
                <div className="min-w-0 space-y-1">
                  <div className="text-sm font-medium leading-5 text-foreground break-words">{item.title}</div>
                  <div className="text-xs text-muted-foreground">Board pricing on 1-tok</div>
                </div>
                <ValueBlock label="Budget" value={item.budget} />
                <ValueBlock label="Low proposal" value={item.currentLow} />
                <ValueBlock label="Spread" value={spread(item.budget, item.currentLow)} />
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

        <div className="bg-secondary">
          <div className="px-4 py-3 text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground">
            Latest proposals
          </div>
          <div className="grid gap-px bg-background/50">
            {proposals.map((proposal) => (
              <div key={`${proposal.provider}-${proposal.request}`} className="flex items-center justify-between gap-3 bg-card px-4 py-4">
                <div className="space-y-1">
                  <div className="text-sm font-medium text-foreground">{proposal.provider}</div>
                  <div className="text-xs text-muted-foreground">{proposal.request}</div>
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
    <div className={`bg-card px-4 py-3 shadow-[0_20px_40px_rgba(0,0,0,0.06)] ${className ?? ""}`}>
      <div className="inline-flex items-center gap-1 text-[11px] uppercase tracking-[0.16em] text-muted-foreground">
        {Icon ? <Icon className="size-4 text-primary" /> : null}
        {label}
      </div>
      <div className="mt-2 min-w-0 font-mono text-[1.75rem] font-semibold leading-none tracking-tight tabular-nums text-foreground">{value}</div>
    </div>
  );
}

function ValueBlock({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground md:hidden">{label}</div>
      <div className="font-mono text-xl font-semibold leading-none tracking-tight tabular-nums text-foreground">{value}</div>
    </div>
  );
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
