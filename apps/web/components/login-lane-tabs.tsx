import Link from "next/link";
import { RiArrowRightUpLine, RiLock2Line, RiShieldKeyholeLine, RiStore2Line, RiTaskLine } from "react-icons/ri";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";

const laneConfig = {
  buyer: {
    next: "/buyer",
    label: "Client",
    title: "Post a request and compare incoming proposals",
    detail: "Lead with budget, review proposals, then award the strongest fit.",
    features: ["Post request", "Compare proposals", "Award work"],
    switchHref: "/login?next=/provider",
    switchLabel: "Switch to provider",
    icon: RiStore2Line,
  },
  provider: {
    next: "/provider",
    label: "Provider",
    title: "Browse requests and submit your price",
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
    <Card className="overflow-hidden">
      <CardHeader className="gap-6 border-b border-border/70">
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-3">
            <div className="eyebrow-pill">{lane.label}</div>
            <div className="space-y-3">
              <CardTitle className="font-display text-[2.4rem] leading-[0.98]">{lane.title}</CardTitle>
              <CardDescription className="max-w-[34ch] text-base leading-7">{lane.detail}</CardDescription>
            </div>
          </div>
          <div className="flex size-11 items-center justify-center rounded-full bg-[var(--ink-accent-weak)] text-primary">
            <Icon className="size-5" />
          </div>
        </div>
        <div className="grid gap-3 sm:grid-cols-3">
          {lane.features.map((feature) => (
            <div key={feature} className="border-t border-border/70 pt-3 text-sm leading-6 text-foreground">
              {feature}
            </div>
          ))}
        </div>
      </CardHeader>
      <CardContent className="space-y-8 p-6">
        <form action="/auth/login" method="post" className="space-y-5">
          <input type="hidden" name="next" value={lane.next} />
          <label className="grid gap-2">
            <span className="text-[11px] font-semibold uppercase tracking-[0.22em] text-muted-foreground">Email</span>
            <Input name="email" type="email" autoComplete="email" placeholder="name@example.com" required />
          </label>
          <label className="grid gap-2">
            <span className="text-[11px] font-semibold uppercase tracking-[0.22em] text-muted-foreground">Password</span>
            <Input name="password" type="password" autoComplete="current-password" placeholder="Enter your password" required />
          </label>

          {error ? <div className="rounded-[1rem] bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</div> : null}

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

        <div className="flex items-center justify-between border-t border-border/70 pt-5 text-sm">
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
