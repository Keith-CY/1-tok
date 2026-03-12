import { redirectToPortal, normalizeDateTimeInput, postGatewayJSON, readRequestPortalViewer } from "../../../lib/marketplace-actions";

export async function POST(request: Request) {
  const viewer = await readRequestPortalViewer(request, "buyer");
  if (!viewer) {
    return redirectToPortal(request, "/login?next=%2Fbuyer");
  }

  const form = await request.formData();
  const title = String(form.get("title") ?? "").trim();
  const category = String(form.get("category") ?? "agent-ops").trim();
  const scope = String(form.get("scope") ?? "").trim();
  const budgetCents = Number.parseInt(String(form.get("budgetCents") ?? "0"), 10);
  const responseDeadlineAt = normalizeDateTimeInput(String(form.get("responseDeadlineAt") ?? ""));

  if (!title || !category || !scope || !Number.isFinite(budgetCents) || budgetCents <= 0 || !responseDeadlineAt) {
    return redirectToPortal(request, "/buyer", "rfq-invalid");
  }

  try {
    await postGatewayJSON("/api/v1/rfqs", viewer.token, {
      buyerOrgId: viewer.membership.organization.id,
      title,
      category,
      scope,
      budgetCents,
      responseDeadlineAt,
    });
    return redirectToPortal(request, "/buyer");
  } catch {
    return redirectToPortal(request, "/buyer", "rfq-create-failed");
  }
}
