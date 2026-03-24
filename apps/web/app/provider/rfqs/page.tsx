import Link from "next/link";
import { RiArrowRightUpLine } from "react-icons/ri";

import { formatMoney } from "@1tok/contracts";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { getProviderDashboardData } from "@/lib/api";
import { requirePortalViewer } from "@/lib/viewer";

export const metadata = {
  title: "Provider Requests",
};

export const dynamic = "force-dynamic";

export default async function ProviderRFQsPage() {
  const viewer = await requirePortalViewer("provider", "/provider/rfqs");
  const data = await getProviderDashboardData({
    authToken: viewer.token,
    providerOrgId: viewer.membership.organization.id,
    requireLive: true,
  });

  const available = [...data.marketOpportunities]
    .filter((item) => !item.hasProviderBid)
    .sort((left, right) => right.budgetCents - left.budgetCents || Date.parse(left.responseDeadlineAt) - Date.parse(right.responseDeadlineAt));

  return (
    <WorkspaceShell
      role="provider"
      title="All open requests"
      description="The full 1-tok request board, sorted for fast pricing decisions."
      actions={[
        { href: "/provider", label: "Back to marketplace", variant: "outline" },
        { href: "/provider/proposals", label: "My proposals" },
      ]}
    >
      <SectionCard eyebrow="All open requests" title="Full request feed" description="Budget first, then live proposal pressure, then delivery window.">
        <div className="space-y-3">
          {available.length === 0 ? (
            <Card className="border-dashed p-6 text-sm text-muted-foreground">No open requests right now.</Card>
          ) : (
            available.map((item, index) => (
              <div key={item.id} className="market-row">
                <div className="grid gap-4 xl:grid-cols-[1.5fr_0.72fr_0.72fr_0.72fr_auto] xl:items-center">
                  <div className="space-y-2">
                    <div className="text-xs text-primary">{marketSignal(item.budgetCents, item.proposalCount, index)}</div>
                    <h3 className="text-xl font-semibold text-balance">{item.title}</h3>
                  </div>
                  <div>
                    <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">Budget</div>
                    <div className="price-inline mt-2">{formatMoney(item.budgetCents)}</div>
                  </div>
                  <div>
                    <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">Current low</div>
                    <div className="mt-2 font-mono text-xl font-semibold tracking-tight tabular-nums text-foreground">
                      {item.lowestQuoteCents ? formatMoney(item.lowestQuoteCents) : "No proposal yet"}
                    </div>
                  </div>
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
    </WorkspaceShell>
  );
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric" }).format(new Date(value));
}

function formatProposalCount(count: number) {
  return count === 1 ? "1 live proposal" : `${count} live proposals`;
}

function marketSignal(budgetCents: number, proposalCount: number, index: number) {
  if (index === 0) return "Top budget now";
  if (proposalCount >= 6) return "Crowded pricing";
  if (budgetCents >= 500000) return "High budget";
  if (proposalCount === 0) return "First proposal wins";
  return "Open for pricing";
}
