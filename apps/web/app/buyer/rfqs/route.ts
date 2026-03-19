import { redirectToPortal, normalizeDateTimeInput, postGatewayJSON, readRequestPortalViewer } from "../../../lib/marketplace-actions";
import { parseCurrencyInputToCents } from "../../../lib/currency";

export async function POST(request: Request) {
  const viewer = await readRequestPortalViewer(request, "buyer");
  if (!viewer) {
    return redirectToPortal(request, "/login?next=%2Fbuyer");
  }

  const form = await request.formData();
  const title = String(form.get("title") ?? "").trim();
  const category = String(form.get("category") ?? "service-request").trim() || "service-request";
  const rawScope = String(form.get("scope") ?? "").trim();
  const scope = rawScope || title;
  const budgetDollars = String(form.get("budgetDollars") ?? "").trim();
  const legacyBudgetCents = Number.parseInt(String(form.get("budgetCents") ?? "0"), 10);
  const budgetCents = budgetDollars ? parseCurrencyInputToCents(budgetDollars) : Number.isFinite(legacyBudgetCents) ? legacyBudgetCents : null;
  const responseDeadlineAt = normalizeDateTimeInput(String(form.get("responseDeadlineAt") ?? ""));

  if (!title || !category || !scope || budgetCents === null || budgetCents <= 0 || !responseDeadlineAt) {
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
