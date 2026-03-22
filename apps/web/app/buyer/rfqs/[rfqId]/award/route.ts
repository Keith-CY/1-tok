import { redirectToPortal, postGatewayJSON, readRequestPortalViewer } from "../../../../../lib/marketplace-actions";

export async function POST(request: Request, { params }: { params: Promise<{ rfqId: string }> }) {
  const viewer = await readRequestPortalViewer(request, "buyer");
  if (!viewer) {
    return redirectToPortal(request, "/login?next=%2Fbuyer");
  }

  const { rfqId } = await params;
  const form = await request.formData();
  const bidId = String(form.get("bidId") ?? "").trim();
  const fundingMode = String(form.get("fundingMode") ?? "prepaid").trim();
  const creditLineId = String(form.get("creditLineId") ?? "").trim();

  if (!rfqId || !bidId || !fundingMode) {
    return redirectToPortal(request, "/buyer", "award-invalid");
  }

  try {
    const payload: Record<string, string> = {
      bidId,
      fundingMode,
    };
    if (creditLineId) {
      payload.creditLineId = creditLineId;
    }
    await postGatewayJSON(`/api/v1/rfqs/${rfqId}/award`, viewer.token, payload);
    return redirectToPortal(request, "/buyer");
  } catch {
    return redirectToPortal(request, "/buyer", "award-failed");
  }
}
