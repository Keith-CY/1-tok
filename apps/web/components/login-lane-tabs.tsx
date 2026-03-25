import Link from "next/link";
import { RiArrowRightUpLine, RiLock2Line, RiShieldKeyholeLine, RiStore2Line, RiTaskLine } from "react-icons/ri";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";

const laneConfig = {
  buyer: {
    next: "/buyer",
    label: "Client",
    title: "Enter your client account",
    detail: "Lead with budget, review proposals, then award the strongest fit.",
    features: ["Post request", "Compare proposals", "Award work"],
    switchHref: "/login?next=/provider",
    switchLabel: "Switch to provider",
    icon: RiStore2Line,
  },
  provider: {
    next: "/provider",
    label: "Provider",
    title: "Enter your provider account",
    detail: "Watch budgets, delivery windows, and proposal counts before you bid.",
    features: ["Browse requests", "Submit proposal", "Track awards"],
    switchHref: "/login?next=/buyer",
    switchLabel: "Switch to client",
    icon: RiTaskLine,
  },
} as const;

export function LoginLaneTabs({
  error,
  initialNext,
}: {
  error: string | null;
  initialNext: string;
}) {
  const lane = initialNext.startsWith("/provider") ? laneConfig.provider : laneConfig.buyer;
  const Icon = lane.icon;

  return (
    <Card className="bg-card shadow-[0_20px_40px_rgba(0,0,0,0.06)]">
      <CardHeader className="gap-6 bg-secondary p-6 sm:p-7">
        <div className="space-y-4">
          <div className="flex items-center justify-between gap-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">{lane.label} account</div>
            <div className="flex size-11 items-center justify-center bg-card text-primary shadow-[0_20px_40px_rgba(0,0,0,0.06)]">
              <Icon className="size-5" />
            </div>
          </div>
          <div className="space-y-2">
            <CardTitle className="max-w-xl text-[clamp(2rem,3vw,2.8rem)] leading-[1.02]">{lane.title}</CardTitle>
            <CardDescription className="max-w-lg text-sm leading-7">{lane.detail}</CardDescription>
          </div>
        </div>
        <div className="grid gap-px bg-background/50 sm:grid-cols-3">
          {lane.features.map((feature) => (
            <div
              key={feature}
              className="bg-card px-3 py-3 text-[10px] font-semibold uppercase tracking-[0.12em] whitespace-nowrap text-foreground shadow-[0_20px_40px_rgba(0,0,0,0.06)] sm:shadow-none"
            >
              {feature}
            </div>
          ))}
        </div>
      </CardHeader>
      <CardContent className="space-y-6 p-6 sm:p-7">
        <form action="/auth/login" method="post" className="space-y-4">
          <input type="hidden" name="next" value={lane.next} />
          <label className="grid gap-2">
            <span className="text-sm font-medium text-foreground">Email</span>
            <Input name="email" type="email" autoComplete="email" placeholder="name@example.com" required />
          </label>
          <label className="grid gap-2">
            <span className="text-sm font-medium text-foreground">Password</span>
            <Input name="password" type="password" autoComplete="current-password" placeholder="Enter your password" required />
          </label>

          {error ? <div className="bg-rose-50 px-3 py-3 text-sm text-rose-700">{error}</div> : null}

          <div className="flex flex-wrap items-center gap-3">
            <Button type="submit" className="min-w-[180px] justify-center">
              <RiLock2Line className="size-4" />
              Enter marketplace
            </Button>
            <div className="inline-flex items-center gap-2 text-sm text-muted-foreground">
              <RiShieldKeyholeLine className="size-4" />
              You will land in the right workspace after sign in
            </div>
          </div>
        </form>

        <div className="flex items-center justify-between bg-secondary px-4 py-4 text-sm">
          <span className="text-muted-foreground">Wrong role?</span>
          <Button asChild variant="ghost" size="sm">
            <Link href={lane.switchHref}>
              {lane.switchLabel}
              <RiArrowRightUpLine className="size-4" />
            </Link>
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
