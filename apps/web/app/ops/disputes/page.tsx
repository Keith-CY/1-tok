import { RiFilter3Line, RiShieldCheckLine } from "react-icons/ri";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Field, SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { formatShortDate } from "@/lib/utils";
import { getOpsDashboardData } from "@/lib/api";
import { requirePortalViewer } from "@/lib/viewer";

export const metadata = {
  title: "Ops Disputes",
};

export const dynamic = "force-dynamic";

export default async function OpsDisputesPage({
  searchParams,
}: {
  searchParams?: Promise<{ q?: string; status?: string }>;
}) {
  const viewer = await requirePortalViewer("ops", "/ops/disputes");
  const data = await getOpsDashboardData({ authToken: viewer.token, requireLive: true });
  const params = await searchParams;
  const q = (params?.q ?? "").trim().toLowerCase();
  const status = (params?.status ?? "open").trim().toLowerCase();

  const disputes = data.disputes.filter((item) => (status === "all" ? true : status === "open" ? item.status !== "resolved" : item.status === status) && (!q || item.orderId.toLowerCase().includes(q) || item.milestoneId.toLowerCase().includes(q) || item.reason.toLowerCase().includes(q)));

  return (
    <WorkspaceShell
      role="ops"
      title="Dispute board"
      description="This is the expanded arbitration view. The homepage only shows the most urgent open disputes."
      actions={[
        { href: "/ops", label: "Back to overview", icon: RiShieldCheckLine, variant: "outline" },
      ]}
    >
      <SectionCard eyebrow="Filter" title="Dispute queue" description="Search by order, milestone, buyer, or reason. Resolve directly from the row.">
        <form method="GET" className="grid gap-4 lg:grid-cols-[1.2fr_0.8fr_auto] lg:items-end">
          <Field label="Search">
            <Input name="q" placeholder="Search disputes" defaultValue={q} />
          </Field>
          <Field label="Status">
            <Input name="status" placeholder="open | resolved | all" defaultValue={status === "open" ? "" : status} />
          </Field>
          <Button type="submit">
            <RiFilter3Line className="size-4" />
            Apply
          </Button>
        </form>
      </SectionCard>

      <section className="grid gap-4">
        {disputes.length === 0 ? (
          <Card className="border-dashed bg-secondary/45 p-6 text-sm text-muted-foreground">No disputes match the current filter.</Card>
        ) : (
          disputes.map((dispute) => (
            <Card key={dispute.id} className="bg-white/82 p-5">
              <div className="space-y-4">
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="font-medium text-foreground">{dispute.id}</h3>
                      <Badge variant={dispute.status === "resolved" ? "success" : "danger"}>{dispute.status}</Badge>
                    </div>
                    <p className="mt-1 text-sm text-muted-foreground">Order {dispute.orderId} · Milestone {dispute.milestoneId}</p>
                    <p className="mt-2 text-sm text-foreground">{dispute.reason}</p>
                  </div>
                  <div className="text-right">
                    <div className="font-display text-2xl tracking-[-0.04em]">{dispute.refundCents}</div>
                    <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">refund cents</div>
                    <div className="mt-2 text-xs text-muted-foreground">{dispute.status === "resolved" ? `Resolved ${formatShortDate(dispute.resolvedAt ?? dispute.createdAt)}` : `Opened ${formatShortDate(dispute.createdAt)}`}</div>
                  </div>
                </div>
                {dispute.status !== "resolved" ? (
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
                ) : null}
              </div>
            </Card>
          ))
        )}
      </section>
    </WorkspaceShell>
  );
}
