import { redirect } from "next/navigation";

import { PublicShell } from "@/components/workspace-shell";
import { LoginLaneTabs } from "@/components/login-lane-tabs";
import { RoleShowcase } from "@/components/role-showcase";

export const metadata = {
  title: "Sign In",
};

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
  const isProvider = nextValue.startsWith("/provider");
  const next = isProvider ? "/provider" : "/buyer";

  return (
    <PublicShell>
      <div className="grid flex-1 gap-10 xl:grid-cols-[minmax(0,1fr)_26rem] xl:items-start">
        <div className="flex flex-col gap-8">
          <div className="sheet-stack px-6 py-6 sm:px-7 sm:py-7 xl:pb-6">
            <div className="space-y-4">
              <div className="eyebrow-pill">Marketplace access</div>
              <h1 className="max-w-4xl font-display text-[clamp(3.3rem,6vw,5.9rem)] font-medium leading-[0.96] tracking-[-0.04em] text-balance">
                {isProvider
                  ? "Enter the board. Read the budget. Price the brief."
                  : "Enter the market. Lead with budget. Decide with proposals."}
              </h1>
              <p className="max-w-2xl text-base leading-8 text-muted-foreground text-pretty">
                {isProvider
                  ? "Providers arrive to read visible budgets, compare delivery windows, and respond with a scoped price before work is awarded."
                  : "Buyers arrive to post funded requests. Providers arrive to price against the same board. Most scoped work is framed around a two-hour delivery window."}
              </p>
            </div>
          </div>

          <div
            className="px-6 py-6 shadow-[0_20px_40px_rgba(0,0,0,0.06)] sm:px-7 sm:py-7"
            style={{
              backgroundImage:
                "linear-gradient(180deg, rgba(255,255,255,0.84), rgba(255,255,255,0.94)), url('https://source.unsplash.com/1600x900/?paper,texture,neutral,minimal')",
              backgroundPosition: "center",
              backgroundSize: "cover",
            }}
          >
            <div className="space-y-3">
              <div className="eyebrow-pill">{isProvider ? "Provider lane" : "Client lane"}</div>
              <div className="max-w-3xl font-display text-[clamp(2.1rem,3.1vw,3rem)] font-medium leading-[1.02] tracking-[-0.03em] text-balance">
                {isProvider ? "Read the board and submit your price against it" : "Post a request and compare incoming proposals"}
              </div>
              <p className="max-w-3xl text-sm leading-7 text-muted-foreground text-pretty">
                {isProvider
                  ? "Watch the same request sheet buyers see, then send a proposal that matches the visible budget, scope, and expected turnaround."
                  : "Start from a funded brief, keep the budget visible, and let provider responses line up against the same sheet before you award."}
              </p>
            </div>
          </div>
        </div>
        <div className="lg:sticky lg:top-10">
          <LoginLaneTabs error={error} initialNext={next} />
        </div>
        <div className="xl:col-span-2">
          <RoleShowcase compact />
        </div>
      </div>
    </PublicShell>
  );
}
