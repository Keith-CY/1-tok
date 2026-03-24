import Link from "next/link";
import { RiArrowRightUpLine, RiFilter3Line, RiShieldCheckLine } from "react-icons/ri";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Field, SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { opsApplications } from "@/lib/portal-mocks";
import { requirePortalViewer } from "@/lib/viewer";

export const metadata = {
  title: "Ops Applications",
};

export const dynamic = "force-dynamic";

export default async function OpsApplicationsPage({
  searchParams,
}: {
  searchParams?: Promise<{ q?: string; status?: string }>;
}) {
  await requirePortalViewer("ops", "/ops/applications");
  const params = await searchParams;
  const q = (params?.q ?? "").trim().toLowerCase();
  const status = (params?.status ?? "pending").trim().toLowerCase();

  const applications = opsApplications.filter((application) => (status === "all" ? true : application.status === status) && (!q || application.providerOrg.toLowerCase().includes(q) || application.category.toLowerCase().includes(q) || application.contact.toLowerCase().includes(q) || application.notes.toLowerCase().includes(q)));

  return (
    <WorkspaceShell
      role="ops"
      title="Application review queue"
      description="This stays focused on provider vetting. The only action exposed by default is opening the application detail."
      actions={[
        { href: "/ops", label: "Back to overview", icon: RiShieldCheckLine, variant: "outline" },
        { href: "/ops/disputes", label: "Open disputes", icon: RiArrowRightUpLine, variant: "secondary" },
      ]}
    >
      <SectionCard eyebrow="Filter" title="Provider applications" description="Filter by search and status, then open the application you need to review.">
        <form method="GET" className="grid gap-4 lg:grid-cols-[1.1fr_0.8fr_auto] lg:items-end">
          <Field label="Search">
            <Input name="q" placeholder="Search provider, category, contact" defaultValue={q} />
          </Field>
          <Field label="Status">
            <Input name="status" placeholder="pending | approved | rejected" defaultValue={status === "pending" ? "" : status} />
          </Field>
          <Button type="submit">
            <RiFilter3Line className="size-4" />
            Apply
          </Button>
        </form>
      </SectionCard>

      <section className="grid gap-4">
        {applications.length === 0 ? (
          <Card className="border-dashed bg-secondary/45 p-6 text-sm text-muted-foreground">No applications match the current filter.</Card>
        ) : (
          applications.map((application) => (
            <Card key={application.id} className="bg-white/82 p-5">
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <div className="flex items-center gap-2">
                    <h3 className="font-medium text-foreground">{application.providerOrg}</h3>
                    <Badge variant={application.status === "approved" ? "success" : application.status === "rejected" ? "danger" : "warning"}>{application.status}</Badge>
                  </div>
                  <p className="mt-1 text-sm text-muted-foreground">{application.category} · {application.contact} · submitted {application.submittedAt}</p>
                  <p className="mt-3 text-sm text-foreground">{application.notes}</p>
                </div>
                <Button asChild variant="outline">
                  <Link href={`/ops/applications/${application.id}`}>
                    Open application
                    <RiArrowRightUpLine className="size-4" />
                  </Link>
                </Button>
              </div>
            </Card>
          ))
        )}
      </section>
    </WorkspaceShell>
  );
}
