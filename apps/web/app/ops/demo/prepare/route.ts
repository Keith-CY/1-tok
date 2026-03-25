import { GatewayRequestError, postGatewayJSON, readRequestPortalViewer } from "../../../../lib/marketplace-actions";
import { redirectToPath } from "../../../../lib/redirect";

export async function POST(request: Request) {
  const viewer = await readRequestPortalViewer(request, "ops");
  if (!viewer) {
    return redirectToPath("/internal/login?next=%2Fops");
  }

  try {
    await postGatewayJSON("/api/v1/ops/demo/prepare", viewer.token, {});
    return redirectToPath("/ops?demoPrepared=success");
  } catch (error) {
    const reason = normalizeDemoPrepareErrorMessage(error instanceof Error ? error : null);
    return redirectToPath(`/ops?error=demo-prepare-failed${reason ? `&demoPrepareError=${encodeURIComponent(reason)}` : ""}`);
  }
}

function normalizeDemoPrepareErrorMessage(error: unknown): string {
  if (!(error instanceof GatewayRequestError)) {
    return "";
  }

  const body = typeof error.payload === "object" && error.payload !== null ? (error.payload as { error?: unknown }) : {};
  if (typeof (body as { error?: unknown }).error === "string") {
    return (body as { error: string }).error;
  }

  return `${error.message}`;
}
