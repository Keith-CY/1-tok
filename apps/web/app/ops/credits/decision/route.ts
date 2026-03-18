import { postGatewayJSON, readRequestPortalViewer } from "../../../../lib/marketplace-actions";
import { redirectToPath } from "../../../../lib/redirect";

export async function POST(request: Request) {
  const viewer = await readRequestPortalViewer(request, "ops");
  if (!viewer) {
    return redirectToPath("/internal/login?next=%2Fops");
  }

  const form = await request.formData();
  const completedOrders = Number.parseInt(String(form.get("completedOrders") ?? "0"), 10);
  const disputedOrders = Number.parseInt(String(form.get("disputedOrders") ?? "0"), 10);
  const lifetimeSpendCents = Number.parseInt(String(form.get("lifetimeSpendCents") ?? "0"), 10);
  const successfulPayments = Number.parseInt(String(form.get("successfulPayments") ?? "0"), 10);
  const failedPayments = Number.parseInt(String(form.get("failedPayments") ?? "0"), 10);

  try {
    const response = await postGatewayJSON("/api/v1/credits/decision", viewer.token, {
      completedOrders,
      successfulPayments,
      failedPayments,
      disputedOrders,
      lifetimeSpendCents,
    });
    const payload = (await response.json()) as {
      decision?: {
        approved?: boolean;
        recommendedLimitCents?: number;
        reason?: string;
      };
    };

    const nextURL = new URL("/ops", request.url);
    nextURL.searchParams.set("creditApproved", String(Boolean(payload.decision?.approved)));
    nextURL.searchParams.set("recommendedLimitCents", String(payload.decision?.recommendedLimitCents ?? 0));
    if (payload.decision?.reason) {
      nextURL.searchParams.set("creditReason", payload.decision.reason);
    }
    return redirectToPath(`${nextURL.pathname}${nextURL.search}${nextURL.hash}`);
  } catch {
    return redirectToPath("/ops?error=credit-decision-failed");
  }
}
