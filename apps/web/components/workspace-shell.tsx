import Link from "next/link";
import type { ReactNode } from "react";
import type { IconType } from "react-icons";
import {
  RiAddLine,
  RiFileList3Line,
  RiFolderChartLine,
  RiLogoutBoxRLine,
  RiShieldCheckLine,
  RiStore2Line,
  RiTaskLine,
} from "react-icons/ri";

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
  icon: IconType;
};

const shellConfig: Record<
  WorkspaceRole,
  {
    label: string;
    subtitle: string;
    icon: IconType;
    homeHref: string;
    nav: NavItem[];
  }
> = {
  buyer: {
    label: "Client",
    subtitle: "Post requests and compare proposals",
    icon: RiStore2Line,
    homeHref: "/buyer",
    nav: [
      { href: "/buyer", label: "Requests", icon: RiFolderChartLine },
      { href: "/buyer/rfqs/create", label: "Post request", icon: RiAddLine },
      { href: "/buyer/orders/ord_1", label: "Awarded work", icon: RiTaskLine },
    ],
  },
  provider: {
    label: "Provider",
    subtitle: "Watch budgets and submit proposals",
    icon: RiTaskLine,
    homeHref: "/provider",
    nav: [
      { href: "/provider", label: "Marketplace", icon: RiStore2Line },
      { href: "/provider/rfqs", label: "Open requests", icon: RiFileList3Line },
      { href: "/provider/proposals", label: "My proposals", icon: RiFolderChartLine },
    ],
  },
  ops: {
    label: "Operations",
    subtitle: "Internal only",
    icon: RiShieldCheckLine,
    homeHref: "/ops",
    nav: [
      { href: "/ops", label: "Queue", icon: RiStore2Line },
      { href: "/ops/applications", label: "Applications", icon: RiFileList3Line },
      { href: "/ops/disputes", label: "Disputes", icon: RiFolderChartLine },
    ],
  },
};

export function PublicShell({ children, mode = "public" }: { children: ReactNode; mode?: "public" | "internal" }) {
  const isInternal = mode === "internal";

  return (
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto flex min-h-dvh max-w-7xl flex-col px-4 py-6 sm:px-6 lg:px-8">
        <header className="mb-12 flex flex-wrap items-center justify-between gap-4 border-b border-border pb-5">
          <Link href={isInternal ? "/internal/login" : "/"} className="space-y-1">
            <div className="text-xs font-medium text-primary">1-tok</div>
            <div className="text-lg font-semibold text-foreground">{isInternal ? "Operations" : "Marketplace"}</div>
          </Link>
          <div className="flex flex-wrap gap-2">
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
  const RoleIcon = config.icon;
  const logoutTarget = role === "ops" ? "/internal/login" : "/login";

  return (
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <header className="mb-10 border-b border-border pb-5">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="flex flex-wrap items-center gap-4 lg:gap-8">
              <Link href={config.homeHref} className="space-y-1">
                <div className="text-xs font-medium text-primary">1-tok</div>
                <div className="text-lg font-semibold text-foreground">Marketplace</div>
              </Link>
              <nav className="hidden flex-wrap items-center gap-2 md:flex">
                {config.nav.map((item) => {
                  const Icon = item.icon;
                  return (
                    <Link
                      key={`${item.href}:${item.label}`}
                      href={item.href}
                      className="inline-flex items-center gap-2 rounded-md border border-transparent px-3 py-2 text-sm text-muted-foreground transition-[background-color,color,border-color] duration-150 hover:border-border hover:bg-secondary hover:text-foreground"
                    >
                      <Icon className="size-4" />
                      {item.label}
                    </Link>
                  );
                })}
              </nav>
            </div>
            <div className="flex items-center gap-3">
              <div className="hidden items-center gap-2 rounded-md border border-border bg-card px-3 py-2 text-xs text-muted-foreground shadow-sm md:inline-flex">
                <RoleIcon className="size-4 text-primary" />
                <span>{config.label}</span>
              </div>
              <form action="/auth/logout" method="post">
                <input type="hidden" name="next" value={logoutTarget} />
                <Button type="submit" variant="outline" size="sm">
                  Sign out
                  <RiLogoutBoxRLine className="size-4" />
                </Button>
              </form>
            </div>
          </div>

          <div className="mt-6 flex gap-2 overflow-x-auto pb-1 md:hidden">
            {config.nav.map((item) => {
              const Icon = item.icon;
              return (
                <Link
                  key={`${item.href}:${item.label}`}
                  href={item.href}
                  className="inline-flex shrink-0 items-center gap-2 rounded-md border border-border bg-card px-3 py-2 text-sm text-muted-foreground shadow-sm"
                >
                  <Icon className="size-4" />
                  {item.label}
                </Link>
              );
            })}
          </div>

          <div className="mt-8 flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div className="max-w-4xl space-y-4">
              <div className="eyebrow-pill">
                <RoleIcon className="size-3.5" />
                <span>{config.label}</span>
                {status ? <span className="text-muted-foreground">· {status}</span> : null}
              </div>
              <div className="space-y-3">
                <h1 className="max-w-4xl text-4xl font-semibold leading-tight text-balance sm:text-5xl">{title}</h1>
                <p className="max-w-2xl text-base leading-7 text-muted-foreground text-pretty">{description}</p>
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
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
          </div>
        </header>

        <div className="space-y-6">{children}</div>
      </div>
    </main>
  );
}

export function StatCard({
  label,
  value,
  detail,
  icon: Icon,
  tone = "default",
}: {
  label: string;
  value: string;
  detail: string;
  icon: IconType;
  tone?: "default" | "success" | "warning" | "danger";
}) {
  const toneClass = {
    default: "bg-secondary text-primary",
    success: "bg-emerald-50 text-emerald-700",
    warning: "bg-amber-50 text-amber-700",
    danger: "bg-rose-50 text-rose-700",
  }[tone];

  return (
    <Card className="market-card">
      <CardContent className="flex items-start justify-between gap-4 p-6">
        <div className="space-y-2">
          <div className="text-xs text-muted-foreground">{label}</div>
          <div className="font-mono text-3xl font-semibold tabular-nums text-foreground">{value}</div>
          <p className="text-sm leading-6 text-muted-foreground text-pretty">{detail}</p>
        </div>
        <div className={cn("flex size-10 items-center justify-center rounded-md", toneClass)}>
          <Icon className="size-5" />
        </div>
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
    <Card>
      <CardHeader className="gap-4 border-b border-border/80">
        <div className="flex items-start justify-between gap-3">
          <div className="space-y-2">
            {eyebrow ? <div className="text-xs font-medium text-primary">{eyebrow}</div> : null}
            <CardTitle className="text-2xl">{title}</CardTitle>
            {description ? <CardDescription>{description}</CardDescription> : null}
          </div>
          {action}
        </div>
      </CardHeader>
      <CardContent className="p-6">{children}</CardContent>
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
    <label className="grid gap-2">
      <span className="text-sm font-medium text-foreground">{label}</span>
      {children}
      {hint ? <span className="text-xs leading-5 text-muted-foreground text-pretty">{hint}</span> : null}
    </label>
  );
}

export function MetaLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4 border-t border-border/80 pt-3 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium text-foreground">{value}</span>
    </div>
  );
}

export function DetailChip({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-card px-4 py-4 shadow-sm">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-2 font-mono text-xl font-semibold tabular-nums text-foreground">{value}</div>
    </div>
  );
}
