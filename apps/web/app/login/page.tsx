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
      <div className="grid flex-1 gap-12 lg:grid-cols-[0.96fr_1.04fr] lg:items-start">
        <div className="space-y-8">
          <div className="space-y-5">
            <div className="eyebrow-pill">
              <RiPriceTag3Line className="size-3.5" />
              Marketplace access
            </div>
            <h1 className="max-w-4xl font-display text-6xl leading-[0.94] tracking-tight text-balance text-foreground sm:text-7xl">
              Enter the market. Lead with budget. Decide with legible motion.
            </h1>
            <p className="max-w-2xl text-base leading-8 text-muted-foreground text-pretty">
              Clients publish requests with a visible budget. Providers answer with proposals and delivery windows. Sign in and the right workspace opens as an operational extension of the same system.
            </p>
          </div>
          <RoleShowcase compact />
        </div>
        <LoginLaneTabs error={error} initialNext={next} />
      </div>
    </PublicShell>
  );
}
