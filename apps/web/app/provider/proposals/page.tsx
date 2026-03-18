import Link from "next/link";
import { RiArrowRightUpLine } from "react-icons/ri";

import { formatMoney } from "@1tok/contracts";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { getProviderDashboardData } from "@/lib/api";
import { requirePortalViewer } from "@/lib/viewer";

export const dynamic = "force-dynamic";

export default async function ProviderProposalsPage() {
  const viewer = await requirePortalViewer("provider", "/provider/proposals");
  const data = await getProviderDashboardData({
    authToken: viewer.token,
    providerOrgId: viewer.membership.organization.id,
    requireLive: true,
  });

  const submitted = data.marketQueue.filter((item) => item.providerBidStatus !== "awarded" && item.providerBidStatus !== "rejected");
  const active = data.marketQueue.filter((item) => item.providerBidStatus === "awarded");

  return (
    <WorkspaceShell
      role="provider"
      title="My proposals"
      description="This is where your active pricing sits after it enters a client's comparison set."
      actions={[{ href: "/provider", label: "Back to marketplace", variant: "outline" }]}
    >
      <section className="grid gap-6 xl:grid-cols-2">
        <SectionCard eyebrow="Pending" title="Waiting on the client" description="These proposals are live and still under comparison.">
          <div className="grid gap-4">
            {submitted.length === 0 ? (
              <Card className="border-dashed p-6 text-sm text-muted-foreground">No live proposals waiting for a client decision.</Card>
            ) : (
              submitted.map((item) => (
                <div key={item.id} className="market-card p-5">
                  <div className="flex items-start justify-between gap-3">
                    <div className="space-y-2">
                      <div className="text-xs text-primary">Live proposal</div>
                      <h3 className="text-lg font-semibold text-balance">{item.title}</h3>
                    </div>
                    <div className="font-mono text-xl font-semibold tabular-nums text-foreground">{formatMoney(item.quoteCents)}</div>
                  </div>
                  <div className="mt-4 text-sm text-muted-foreground">Delivery window {formatDate(item.responseDeadlineAt)}</div>
                  <Button asChild variant="outline" className="mt-5 w-full justify-between">
                    <Link href={`/provider/rfqs/${item.rfqId}`}>
                      View request
                      <RiArrowRightUpLine className="size-4" />
                    </Link>
                  </Button>
                </div>
              ))
            )}
          </div>
        </SectionCard>

        <SectionCard eyebrow="Awarded" title="Already won" description="These requests have moved out of pricing and into delivery.">
          <div className="grid gap-4">
            {active.length === 0 ? (
              <Card className="border-dashed p-6 text-sm text-muted-foreground">No awarded requests yet.</Card>
            ) : (
              active.map((item) => (
                <div key={item.id} className="market-card p-5">
                  <div className="flex items-start justify-between gap-3">
                    <div className="space-y-2">
                      <div className="text-xs text-primary">Awarded</div>
                      <h3 className="text-lg font-semibold text-balance">{item.title}</h3>
                    </div>
                    <div className="font-mono text-xl font-semibold tabular-nums text-foreground">{formatMoney(item.quoteCents)}</div>
                  </div>
                  <div className="mt-4 text-sm text-muted-foreground">Delivery window {formatDate(item.responseDeadlineAt)}</div>
                  <Button asChild variant="outline" className="mt-5 w-full justify-between">
                    <Link href={item.orderId ? `/provider/orders/${item.orderId}` : `/provider/rfqs/${item.rfqId}`}>
                      View delivery
                      <RiArrowRightUpLine className="size-4" />
                    </Link>
                  </Button>
                </div>
              ))
            )}
          </div>
        </SectionCard>
      </section>
    </WorkspaceShell>
  );
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric" }).format(new Date(value));
}
