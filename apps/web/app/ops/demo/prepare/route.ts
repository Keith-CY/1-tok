import { postGatewayJSON, readRequestPortalViewer } from "../../../../lib/marketplace-actions";
import { redirectToPath } from "../../../../lib/redirect";

export async function POST(request: Request) {
  const viewer = await readRequestPortalViewer(request, "ops");
  if (!viewer) {
    return redirectToPath("/internal/login?next=%2Fops");
  }

  try {
    await postGatewayJSON("/api/v1/ops/demo/prepare", viewer.token, {});
    return redirectToPath("/ops?demoPrepared=success");
  } catch {
    return redirectToPath("/ops?error=demo-prepare-failed");
  }
}
