import { NextResponse } from "next/server";

import { postGatewayJSON, readRequestPortalViewer } from "../../../../../lib/marketplace-actions";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ disputeId: string }> },
) {
  const viewer = await readRequestPortalViewer(request, "ops");
  if (!viewer) {
    return NextResponse.redirect(new URL("/login?next=%2Fops", request.url), 303);
  }

  const form = await request.formData();
  const resolution = String(form.get("resolution") ?? "").trim();
  if (!resolution) {
    return NextResponse.redirect(new URL("/ops?error=missing-dispute-resolution", request.url), 303);
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
    const nextURL = new URL("/ops", request.url);
    nextURL.searchParams.set("resolvedDisputeId", result.dispute?.id ?? disputeId);
    nextURL.searchParams.set("disputeStatus", result.dispute?.status ?? "resolved");
    return NextResponse.redirect(nextURL, 303);
  } catch {
    return NextResponse.redirect(new URL("/ops?error=dispute-resolution-failed", request.url), 303);
  }
}
