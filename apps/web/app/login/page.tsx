import { redirect } from "next/navigation";
import { RiPriceTag3Line } from "react-icons/ri";

import { PublicShell } from "@/components/workspace-shell";
import { LoginLaneTabs } from "@/components/login-lane-tabs";
import { RoleShowcase } from "@/components/role-showcase";

export const dynamic = "force-dynamic";

const errorMessages: Record<string, string> = {
  "invalid-credentials": "Incorrect email or password.",
  "missing-fields": "Enter both email and password.",
};

export default async function LoginPage({
  searchParams,
}: {
  searchParams?: Promise<{ error?: string; next?: string }>;
}) {
  const params = await searchParams;
  const nextValue = params?.next?.startsWith("/") ? params.next : "/buyer";

  if (nextValue.startsWith("/ops")) {
    redirect(`/internal/login?next=${encodeURIComponent(nextValue)}`);
  }

  const error = params?.error ? errorMessages[params.error] ?? "Sign in failed. Try again." : null;
  const next = nextValue.startsWith("/provider") ? "/provider" : "/buyer";

  return (
    <PublicShell>
      <div className="grid flex-1 gap-10 lg:grid-cols-[1.02fr_0.98fr] lg:items-start">
        <div className="space-y-6">
          <div className="space-y-4">
            <div className="eyebrow-pill">
              <RiPriceTag3Line className="size-3.5" />
              Marketplace access
            </div>
            <h1 className="max-w-4xl text-5xl font-semibold leading-[1.04] text-balance sm:text-6xl">
              Enter the market. Lead with budget. Decide with proposals.
            </h1>
            <p className="max-w-2xl text-base leading-8 text-muted-foreground text-pretty">
              Clients post requests with a live budget. Providers respond with proposals and delivery windows. Sign in to continue where you belong.
            </p>
          </div>
          <RoleShowcase compact />
        </div>
        <LoginLaneTabs error={error} initialNext={next} />
      </div>
    </PublicShell>
  );
}
