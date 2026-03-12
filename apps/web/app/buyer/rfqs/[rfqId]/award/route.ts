import { redirectToPortal, postGatewayJSON, readRequestPortalViewer } from "../../../../../lib/marketplace-actions";

export async function POST(request: Request, context: { params: { rfqId: string } }) {
  const viewer = await readRequestPortalViewer(request, "buyer");
  if (!viewer) {
    return redirectToPortal(request, "/login?next=%2Fbuyer");
  }

  const form = await request.formData();
  const bidId = String(form.get("bidId") ?? "").trim();
  const fundingMode = String(form.get("fundingMode") ?? "credit").trim();
  const creditLineId = String(form.get("creditLineId") ?? "").trim();

  if (!context.params.rfqId || !bidId || !fundingMode) {
    return redirectToPortal(request, "/buyer", "award-invalid");
  }

  try {
    await postGatewayJSON(`/api/v1/rfqs/${context.params.rfqId}/award`, viewer.token, {
      bidId,
      fundingMode,
      creditLineId,
    });
    return redirectToPortal(request, "/buyer");
  } catch {
    return redirectToPortal(request, "/buyer", "award-failed");
  }
}
