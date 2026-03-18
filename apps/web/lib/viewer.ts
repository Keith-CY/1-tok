import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { resolveIAMBaseURL, SESSION_COOKIE_NAME } from "./session";

export type PortalKind = "buyer" | "provider" | "ops";

export interface IAMActorMembership {
  role: string;
  organization: {
    id: string;
    name: string;
    kind: PortalKind;
  };
}

export interface IAMActor {
  user: {
    id: string;
    email: string;
    name: string;
  };
  memberships: IAMActorMembership[];
}

export interface ViewerSession {
  token: string;
  actor: IAMActor;
}

export async function fetchIAMActor(token: string): Promise<IAMActor | null> {
  const baseUrl = resolveIAMBaseURL();
  if (!baseUrl || !token) {
    return null;
  }

  const response = await fetch(`${baseUrl}/v1/me`, {
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
    },
    cache: "no-store",
  });

  if (!response.ok) {
    return null;
  }

  return (await response.json()) as IAMActor;
}

export function findPortalMembership(actor: IAMActor, kind: PortalKind): IAMActorMembership | null {
  return actor.memberships.find((membership) => membership.organization.kind === kind) ?? null;
}

export async function getViewerSession(): Promise<ViewerSession | null> {
  const cookieStore = await cookies();
  const token = cookieStore.get(SESSION_COOKIE_NAME)?.value;
  if (!token) {
    return null;
  }

  const actor = await fetchIAMActor(token);
  if (!actor) {
    return null;
  }

  return { token, actor };
}

export async function requirePortalViewer(kind: PortalKind, nextPath: string) {
  const viewer = await getViewerSession();
  if (!viewer) {
    redirect(`${loginPathFor(kind)}?next=${encodeURIComponent(nextPath)}`);
  }

  const membership = findPortalMembership(viewer.actor, kind);
  if (!membership) {
    redirect(kind === "ops" ? "/internal/login" : "/");
  }

  return {
    ...viewer,
    membership,
  };
}

function loginPathFor(kind: PortalKind) {
  return kind === "ops" ? "/internal/login" : "/login";
}
