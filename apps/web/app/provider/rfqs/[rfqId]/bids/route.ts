import { redirectToPortal, postGatewayJSON, readRequestPortalViewer } from "../../../../../lib/marketplace-actions";

export async function POST(request: Request, { params }: { params: Promise<{ rfqId: string }> }) {
  const viewer = await readRequestPortalViewer(request, "provider");
  if (!viewer) {
    return redirectToPortal(request, "/login?next=%2Fprovider");
  }

  const { rfqId } = await params;
  const form = await request.formData();
  const rawMessage = String(form.get("message") ?? "").trim();
  const message = rawMessage || "We can take this on and deliver within the requested window.";
  const quoteCents = Number.parseInt(String(form.get("quoteCents") ?? "0"), 10);
  const milestoneTitle = String(form.get("milestoneTitle") ?? "Service delivery").trim() || "Service delivery";
  const rawMilestoneBasePriceCents = Number.parseInt(String(form.get("milestoneBasePriceCents") ?? ""), 10);
  const rawMilestoneBudgetCents = Number.parseInt(String(form.get("milestoneBudgetCents") ?? ""), 10);
  const milestoneBasePriceCents =
    Number.isFinite(rawMilestoneBasePriceCents) && rawMilestoneBasePriceCents > 0 ? rawMilestoneBasePriceCents : quoteCents;
  const milestoneBudgetCents =
    Number.isFinite(rawMilestoneBudgetCents) && rawMilestoneBudgetCents > 0
      ? rawMilestoneBudgetCents
      : Math.max(quoteCents, milestoneBasePriceCents);

  if (
    !rfqId ||
    !Number.isFinite(quoteCents) ||
    quoteCents <= 0 ||
    !Number.isFinite(milestoneBasePriceCents) ||
    milestoneBasePriceCents <= 0 ||
    !Number.isFinite(milestoneBudgetCents) ||
    milestoneBudgetCents <= 0
  ) {
    return redirectToPortal(request, "/provider", "bid-invalid");
  }

  try {
    await postGatewayJSON(`/api/v1/rfqs/${rfqId}/bids`, viewer.token, {
      providerOrgId: viewer.membership.organization.id,
      message,
      quoteCents,
      milestones: [
        {
          id: "ms_1",
          title: milestoneTitle,
          basePriceCents: milestoneBasePriceCents,
          budgetCents: milestoneBudgetCents,
        },
      ],
    });
    return redirectToPortal(request, "/provider");
  } catch {
    return redirectToPortal(request, "/provider", "bid-submit-failed");
  }
}
