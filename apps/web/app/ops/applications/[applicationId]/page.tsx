import Link from "next/link";
import { RiArrowLeftLine, RiShieldCheckLine } from "react-icons/ri";

import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { DetailChip, SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { opsApplications } from "@/lib/portal-mocks";
import { requirePortalViewer } from "@/lib/viewer";

export const metadata = {
  title: "Ops Application",
};

export default async function OpsApplicationDetailPage({ params }: { params: Promise<{ applicationId: string }> }) {
  const { applicationId } = await params;
  await requirePortalViewer("ops", "/ops/applications");
  const application = opsApplications.find((item) => item.id === applicationId);

  if (!application) {
    return (
      <WorkspaceShell
        role="ops"
        title="Application not found"
        description="The requested application is not available in this environment."
        actions={[{ href: "/ops/applications", label: "Back to applications", icon: RiArrowLeftLine, variant: "outline" }]}
      >
        <Card className="border-dashed bg-secondary/45 p-6 text-sm text-muted-foreground">Return to the application queue and pick another record.</Card>
      </WorkspaceShell>
    );
  }

  return (
    <WorkspaceShell
      role="ops"
      title={`Application ${application.id}`}
      description="A focused review page with only the application summary you need to triage next steps."
      status={application.status}
      actions={[
        { href: "/ops/applications", label: "Back to applications", icon: RiArrowLeftLine, variant: "outline" },
        { href: "/ops/disputes", label: "Open disputes", icon: RiShieldCheckLine, variant: "secondary" },
      ]}
    >
      <section className="grid gap-4 lg:grid-cols-4">
        <DetailChip label="Provider" value={application.providerOrg} />
        <DetailChip label="Category" value={application.category} />
        <DetailChip label="Contact" value={application.contact} />
        <DetailChip label="Submitted" value={application.submittedAt} />
      </section>

      <SectionCard eyebrow="Review note" title="Why this application is in the queue" description="Keep the detail view focused on the current decision context.">
        <div className="space-y-4">
          <Card className="bg-white/82 p-5 text-sm leading-7 text-foreground">{application.notes}</Card>
          <div className="flex items-center gap-3">
            <Badge variant={application.status === "approved" ? "success" : application.status === "rejected" ? "danger" : "warning"}>{application.status}</Badge>
            <span className="text-sm text-muted-foreground">No live approve / reject route is exposed from this environment yet.</span>
          </div>
        </div>
      </SectionCard>
    </WorkspaceShell>
  );
}
