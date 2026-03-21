import { redirectToPortal, readRequestPortalViewer } from "../../../lib/marketplace-actions";

function resolveSettlementBaseURL(): string | null {
  return process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL?.replace(/\/$/, "") ?? null;
}

export async function POST(request: Request) {
  const viewer = await readRequestPortalViewer(request, "buyer");
  if (!viewer) {
    return redirectToPortal(request, "/login?next=%2Fbuyer");
  }

  const settlementBaseURL = resolveSettlementBaseURL();
  if (!settlementBaseURL) {
    return redirectToPortal(request, "/buyer", "topup-failed");
  }

  const form = await request.formData();
  const amount = String(form.get("amount") ?? "").trim();
  const asset = String(form.get("asset") ?? "USDI").trim();
  if (!amount) {
    return redirectToPortal(request, "/buyer", "topup-failed");
  }

  const response = await fetch(`${settlementBaseURL}/v1/topups`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${viewer.token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ amount, asset }),
    cache: "no-store",
  }).catch(() => null);

  if (!response?.ok) {
    return redirectToPortal(request, "/buyer", "topup-failed");
  }

  return redirectToPortal(request, "/buyer?topup=success");
}
