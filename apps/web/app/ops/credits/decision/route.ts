import { redirectToPath } from "../../../../lib/redirect";
import { postGatewayJSON, readRequestPortalViewer } from "../../../../lib/marketplace-actions";

export async function POST(request: Request) {
	const viewer = await readRequestPortalViewer(request, "ops");
	if (!viewer) {
		return redirectToPath("/login?next=%2Fops");
	}

  const form = await request.formData();
  const payload = {
    completedOrders: parseCount(form.get("completedOrders")),
    successfulPayments: parseCount(form.get("successfulPayments")),
    failedPayments: parseCount(form.get("failedPayments")),
    disputedOrders: parseCount(form.get("disputedOrders")),
    lifetimeSpendCents: parseCount(form.get("lifetimeSpendCents")),
  };

  try {
    const response = await postGatewayJSON("/api/v1/credits/decision", viewer.token, payload);
    const result = (await response.json()) as {
      decision?: {
        approved?: boolean;
        recommendedLimitCents?: number;
        reason?: string;
      };
    };

		const nextURL = new URL("/ops", "http://portal.internal");
		nextURL.searchParams.set("creditApproved", String(Boolean(result.decision?.approved)));
		nextURL.searchParams.set("recommendedLimitCents", String(result.decision?.recommendedLimitCents ?? 0));
		nextURL.searchParams.set("creditReason", result.decision?.reason ?? "Decision unavailable");
		return redirectToPath(`${nextURL.pathname}${nextURL.search}${nextURL.hash}`);
	} catch {
		return redirectToPath("/ops?error=credit-decision-failed");
	}
}

function parseCount(value: FormDataEntryValue | null): number {
  const parsed = Number.parseInt(String(value ?? "0"), 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : 0;
}
