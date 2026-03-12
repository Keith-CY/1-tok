import { redirectToPath } from "../../../../../lib/redirect";
import { postGatewayJSON, readRequestPortalViewer } from "../../../../../lib/marketplace-actions";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ disputeId: string }> },
) {
	const viewer = await readRequestPortalViewer(request, "ops");
	if (!viewer) {
		return redirectToPath("/login?next=%2Fops");
	}

	const form = await request.formData();
	const resolution = String(form.get("resolution") ?? "").trim();
	if (!resolution) {
		return redirectToPath("/ops?error=missing-dispute-resolution");
	}

  const { disputeId } = await params;

  try {
    const response = await postGatewayJSON(`/api/v1/disputes/${disputeId}/resolve`, viewer.token, {
      resolution,
      resolvedBy: viewer.actor.user.id,
    });
    const result = (await response.json()) as {
      dispute?: {
        id?: string;
        status?: string;
      };
    };
		const nextURL = new URL("/ops", "http://portal.internal");
		nextURL.searchParams.set("resolvedDisputeId", result.dispute?.id ?? disputeId);
		nextURL.searchParams.set("disputeStatus", result.dispute?.status ?? "resolved");
		return redirectToPath(`${nextURL.pathname}${nextURL.search}${nextURL.hash}`);
	} catch {
		return redirectToPath("/ops?error=dispute-resolution-failed");
	}
}
