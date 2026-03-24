import Link from "next/link";
import type { ReactNode } from "react";
import type { IconType } from "react-icons";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export type WorkspaceRole = "buyer" | "provider" | "ops";

type ShellAction = {
  href: string;
  label: string;
  icon?: IconType;
  variant?: "default" | "secondary" | "outline" | "ghost" | "soft";
};

type NavItem = {
  href: string;
  label: string;
};

const shellConfig: Record<
  WorkspaceRole,
  {
    label: string;
    subtitle: string;
    homeHref: string;
    nav: NavItem[];
  }
> = {
  buyer: {
    label: "Client",
    subtitle: "Post requests and compare proposals",
    homeHref: "/buyer",
    nav: [
      { href: "/buyer", label: "Requests" },
      { href: "/buyer/rfqs/create", label: "Post request" },
      { href: "/buyer", label: "Delivery" },
    ],
  },
  provider: {
    label: "Provider",
    subtitle: "Watch budgets and submit proposals",
    homeHref: "/provider",
    nav: [
      { href: "/provider", label: "Marketplace" },
      { href: "/provider/rfqs", label: "Open requests" },
      { href: "/provider/proposals", label: "My proposals" },
    ],
  },
  ops: {
    label: "Operations",
    subtitle: "Internal only",
    homeHref: "/ops",
    nav: [
      { href: "/ops", label: "Queue" },
      { href: "/ops/applications", label: "Applications" },
      { href: "/ops/disputes", label: "Disputes" },
    ],
  },
};

export function PublicShell({ children, mode = "public" }: { children: ReactNode; mode?: "public" | "internal" }) {
  const isInternal = mode === "internal";

  return (
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto flex min-h-dvh max-w-[96rem] flex-col px-5 py-6 sm:px-8 lg:px-12">
        <header className="mb-16 grid gap-8 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start">
          <div className="space-y-4">
            <Link href={isInternal ? "/internal/login" : "/"} className="block space-y-3">
              <div className="eyebrow-pill">1-tok</div>
              <div className="space-y-2">
                <div className="font-display text-[clamp(2.5rem,5vw,4.5rem)] font-medium leading-[0.98] tracking-[-0.03em] text-balance">
                  {isInternal ? "Operations broadsheet" : "Marketplace broadsheet"}
                </div>
                <p className="max-w-2xl text-sm leading-7 text-muted-foreground">
                  {isInternal
                    ? "Internal views stay stripped down to queue, posture, and direct action."
                    : "A market view that treats pricing, timing, and delivery as editorial facts instead of app chrome."}
                </p>
              </div>
            </Link>
          </div>
          <div className="flex flex-wrap items-start justify-start gap-3 lg:justify-end">
            {isInternal ? (
              <Button asChild variant="outline" size="sm">
                <Link href="/">Back to marketplace</Link>
              </Button>
            ) : (
              <>
                <Button asChild variant="outline" size="sm">
                  <Link href="/login?next=/buyer">Client sign in</Link>
                </Button>
                <Button asChild size="sm">
                  <Link href="/login?next=/provider">Provider sign in</Link>
                </Button>
              </>
            )}
          </div>
        </header>
        {children}
      </div>
    </main>
  );
}

export function WorkspaceShell({
  role,
  title,
  description,
  actions = [],
  status,
  children,
}: {
  role: WorkspaceRole;
  title: string;
  description: string;
  actions?: ShellAction[];
  status?: string;
  children: ReactNode;
}) {
  const config = shellConfig[role];
  const logoutTarget = role === "ops" ? "/internal/login" : "/login";

  return (
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto max-w-[96rem] px-5 py-6 sm:px-8 lg:px-12">
        <div className="grid gap-10 lg:grid-cols-[16rem_minmax(0,1fr)] lg:items-start">
          <aside className="sheet-stack px-5 py-6 sm:px-6">
            <div className="space-y-8">
              <Link href={config.homeHref} className="block space-y-3">
                <div className="eyebrow-pill">1-tok</div>
                <div className="space-y-2">
                  <div className="font-display text-3xl font-medium leading-[1.02] tracking-[-0.03em]">{config.label}</div>
                  <p className="text-sm leading-7 text-muted-foreground">{config.subtitle}</p>
                </div>
              </Link>

              <nav className="grid gap-2">
                {config.nav.map((item) => (
                  <Link
                    key={`${item.href}:${item.label}`}
                    href={item.href}
                    className="inline-flex min-h-11 items-center bg-transparent px-0 py-1 text-sm text-muted-foreground transition-colors duration-150 hover:text-foreground"
                  >
                    <span className="border-b border-transparent pb-1 hover:border-black/25">{item.label}</span>
                  </Link>
                ))}
              </nav>

              <form action="/auth/logout" method="post" className="pt-4">
                <input type="hidden" name="next" value={logoutTarget} />
                <Button type="submit" variant="ghost" size="sm" className="justify-start">
                  Sign out
                </Button>
              </form>
            </div>
          </aside>

          <div className="space-y-10">
            <header className="grid gap-8 xl:grid-cols-[minmax(0,1fr)_auto] xl:items-end">
              <div className="space-y-5">
                <div className="eyebrow-pill">
                  {config.label}
                  {status ? <span className="text-muted-foreground">/ {status}</span> : null}
                </div>
                <div className="space-y-4">
                  <h1 className="max-w-5xl font-display text-[clamp(3.25rem,7vw,6rem)] font-medium leading-[0.96] tracking-[-0.03em] text-balance">
                    {title}
                  </h1>
                  <p className="max-w-3xl text-base leading-8 text-muted-foreground text-pretty">{description}</p>
                </div>
              </div>
              <div className="flex flex-wrap items-start gap-3 xl:justify-end">
                {actions.map((action) => {
                  const Icon = action.icon;
                  return (
                    <Button key={action.href} asChild variant={action.variant ?? "outline"}>
                      <Link href={action.href}>
                        {Icon ? <Icon className="size-4" /> : null}
                        {action.label}
                      </Link>
                    </Button>
                  );
                })}
              </div>
            </header>

            <div className="space-y-8">{children}</div>
          </div>
        </div>
      </div>
    </main>
  );
}

export function StatCard({
  label,
  value,
  detail,
  icon: _Icon,
  tone = "default",
}: {
  label: string;
  value: string;
  detail: string;
  icon: IconType;
  tone?: "default" | "success" | "warning" | "danger";
}) {
  const toneClass = {
    default: "text-foreground",
    success: "text-accent",
    warning: "text-accent",
    danger: "text-destructive",
  }[tone];

  return (
    <Card className="bg-secondary">
      <CardContent className="space-y-3 p-6">
        <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">{label}</div>
        <div className={cn("font-mono text-3xl font-semibold tabular-nums", toneClass)}>{value}</div>
        <p className="max-w-sm text-sm leading-7 text-muted-foreground text-pretty">{detail}</p>
      </CardContent>
    </Card>
  );
}

export function SectionCard({
  eyebrow,
  title,
  description,
  action,
  children,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Card className="bg-secondary">
      <CardHeader className="gap-5 p-8 pb-0">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
          <div className="space-y-3">
            {eyebrow ? <div className="eyebrow-pill">{eyebrow}</div> : null}
            {title ? <CardTitle>{title}</CardTitle> : null}
            {description ? <CardDescription>{description}</CardDescription> : null}
          </div>
          {action ? <div className="shrink-0">{action}</div> : null}
        </div>
      </CardHeader>
      <CardContent className="p-8">{children}</CardContent>
    </Card>
  );
}

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="grid gap-3">
      <span className="text-[11px] font-bold uppercase tracking-[0.20em] text-foreground/90">{label}</span>
      {children}
      {hint ? <span className="text-sm leading-7 text-muted-foreground text-pretty">{hint}</span> : null}
    </label>
  );
}

export function MetaLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid gap-1 bg-card px-4 py-4 text-sm sm:grid-cols-[12rem_minmax(0,1fr)] sm:items-start">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium text-foreground">{value}</span>
    </div>
  );
}

export function DetailChip({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-card px-5 py-5">
      <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">{label}</div>
      <div className="mt-3 font-mono text-xl font-semibold tabular-nums text-foreground">{value}</div>
    </div>
  );
}
