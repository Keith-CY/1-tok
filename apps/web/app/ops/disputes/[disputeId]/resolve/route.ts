import { postGatewayJSON, readRequestPortalViewer } from "../../../../../lib/marketplace-actions";
import { redirectToPath } from "../../../../../lib/redirect";

export async function POST(request: Request, { params }: { params: Promise<{ disputeId: string }> }) {
  const viewer = await readRequestPortalViewer(request, "ops");
  if (!viewer) {
    return redirectToPath("/internal/login?next=%2Fops");
  }

  const { disputeId } = await params;
  const form = await request.formData();
  const resolution = String(form.get("resolution") ?? "").trim();

  if (!disputeId || !resolution) {
    return redirectToPath("/ops?error=missing-dispute-resolution");
  }

  try {
    const response = await postGatewayJSON(`/api/v1/disputes/${disputeId}/resolve`, viewer.token, {
      resolution,
      resolvedBy: viewer.actor.user.id,
    });
    const payload = (await response.json()) as {
      dispute?: {
        status?: string;
      };
    };
    const nextURL = new URL("/ops", request.url);
    nextURL.searchParams.set("resolvedDisputeId", disputeId);
    nextURL.searchParams.set("disputeStatus", payload.dispute?.status ?? "resolved");
    return redirectToPath(`${nextURL.pathname}${nextURL.search}${nextURL.hash}`);
  } catch {
    return redirectToPath("/ops?error=dispute-resolution-failed");
  }
}
