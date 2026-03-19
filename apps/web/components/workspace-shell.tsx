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
import { WorkspaceNav, type WorkspaceNavItem } from "@/components/workspace-nav";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export type WorkspaceRole = "buyer" | "provider" | "ops";

type ShellAction = {
  href: string;
  label: string;
  icon?: IconType;
  variant?: "default" | "secondary" | "outline" | "ghost" | "soft";
};

const shellConfig: Record<
  WorkspaceRole,
  {
    label: string;
    subtitle: string;
    icon: IconType;
    homeHref: string;
    nav: WorkspaceNavItem[];
  }
> = {
  buyer: {
    label: "Client",
    subtitle: "Post requests and compare proposals",
    icon: RiStore2Line,
    homeHref: "/buyer",
    nav: [
      { href: "/buyer", label: "Requests", icon: "folderChart" },
      { href: "/buyer/rfqs/create", label: "Post request", icon: "add" },
      { href: "/buyer", label: "Delivery", icon: "task" },
    ],
  },
  provider: {
    label: "Provider",
    subtitle: "Watch budgets and submit proposals",
    icon: RiTaskLine,
    homeHref: "/provider",
    nav: [
      { href: "/provider", label: "Marketplace", icon: "store" },
      { href: "/provider/rfqs", label: "Open requests", icon: "fileList" },
      { href: "/provider/proposals", label: "My proposals", icon: "folderChart" },
    ],
  },
  ops: {
    label: "Operations",
    subtitle: "Internal only",
    icon: RiShieldCheckLine,
    homeHref: "/ops",
    nav: [
      { href: "/ops", label: "Queue", icon: "store" },
      { href: "/ops/applications", label: "Applications", icon: "fileList" },
      { href: "/ops/disputes", label: "Disputes", icon: "folderChart" },
    ],
  },
};

export function PublicShell({ children, mode = "public" }: { children: ReactNode; mode?: "public" | "internal" }) {
  const isInternal = mode === "internal";

  return (
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto flex min-h-dvh max-w-[1520px] flex-col px-4 pb-10 pt-4 sm:px-6 lg:px-8">
        <header className="sticky top-4 z-20 mb-12 flex h-16 flex-wrap items-center justify-between gap-4 rounded-full border border-border/70 bg-background/82 px-5 backdrop-blur-xl">
          <Link href={isInternal ? "/internal/login" : "/"} className="flex min-w-0 flex-col">
            <div className="text-[10px] font-semibold uppercase tracking-[0.28em] text-primary">1-tok</div>
            <div className="font-display text-[1.3rem] leading-none text-foreground">{isInternal ? "Operations" : "Marketplace"}</div>
          </Link>
          <div className="flex flex-wrap items-center gap-2">
            <div className="hidden text-[11px] font-semibold uppercase tracking-[0.22em] text-muted-foreground md:inline-flex">
              {isInternal ? "Operational mode" : "Editorial mode"}
            </div>
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
        <div className="flex-1">{children}</div>
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
      <div className="mx-auto flex min-h-dvh max-w-[1560px] flex-col gap-8 px-4 py-5 sm:px-6 lg:flex-row lg:gap-12 lg:px-8">
        <aside className="hidden lg:flex lg:w-[220px] lg:flex-col lg:pb-4 lg:pt-3">
          <div className="space-y-8">
            <Link href={config.homeHref} className="block space-y-2">
              <div className="text-[10px] font-semibold uppercase tracking-[0.28em] text-primary">1-tok</div>
              <div className="font-display text-[1.9rem] leading-none text-foreground">Marketplace</div>
              <div className="max-w-[18ch] text-sm leading-6 text-muted-foreground">{config.subtitle}</div>
            </Link>

            <div className="rounded-[1.25rem] border border-border/70 bg-card/85 p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">Account</div>
              <div className="mt-3 space-y-3">
                <div className="text-sm leading-6 text-muted-foreground">Keep role switching visible and intentional from the top of the workspace.</div>
                <form action="/auth/logout" method="post">
                  <input type="hidden" name="next" value={logoutTarget} />
                  <Button type="submit" variant="outline" size="sm" className="w-full justify-between">
                    Sign out
                    <RiLogoutBoxRLine className="size-4" />
                  </Button>
                </form>
              </div>
            </div>

            <div className="rounded-[1.25rem] border border-border/70 bg-card/85 p-4">
              <div className="inline-flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.22em] text-primary">
                <RoleIcon className="size-4" />
                {config.label}
              </div>
              {status ? <div className="mt-3 text-sm leading-6 text-muted-foreground">{status}</div> : null}
            </div>

            <WorkspaceNav items={config.nav} />
          </div>
        </aside>

        <div className="min-w-0 flex-1">
          <header className="border-b border-border/70 pb-8">
            <div className="flex items-center justify-between gap-3 lg:hidden">
              <Link href={config.homeHref} className="flex min-w-0 flex-col">
                <div className="text-[10px] font-semibold uppercase tracking-[0.28em] text-primary">1-tok</div>
                <div className="font-display text-[1.35rem] leading-none text-foreground">{config.label}</div>
              </Link>
              <form action="/auth/logout" method="post">
                <input type="hidden" name="next" value={logoutTarget} />
                <Button type="submit" variant="outline" size="sm">
                  Sign out
                  <RiLogoutBoxRLine className="size-4" />
                </Button>
              </form>
            </div>

            <div className="mt-6 flex flex-col gap-6 lg:mt-0 lg:flex-row lg:items-end lg:justify-between">
              <div className="max-w-4xl space-y-4">
                <div className="eyebrow-pill">
                  <RoleIcon className="size-3.5" />
                  <span>{config.label}</span>
                  {status ? <span className="text-muted-foreground">/ {status}</span> : null}
                </div>
                <div className="space-y-4">
                  <h1 className="max-w-4xl font-display text-5xl leading-[0.98] tracking-tight text-balance text-foreground sm:text-6xl">
                    {title}
                  </h1>
                  <p className="max-w-3xl text-base leading-8 text-muted-foreground text-pretty">{description}</p>
                </div>
              </div>
              <div className="flex flex-wrap gap-3">
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

            <div className="mt-6 lg:hidden">
              <WorkspaceNav items={config.nav} mobile />
            </div>
          </header>

          <div className="space-y-8 py-8">{children}</div>
        </div>
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
    default: "text-primary",
    success: "text-emerald-700",
    warning: "text-amber-700",
    danger: "text-rose-700",
  }[tone];

  return (
    <Card className="border-border/70 bg-card/90">
      <CardContent className="space-y-5 p-6">
        <div className="flex items-center justify-between gap-4">
          <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">{label}</div>
          <Icon className={cn("size-5", toneClass)} />
        </div>
        <div className="font-display text-4xl leading-none tracking-tight text-foreground">{value}</div>
        <p className="max-w-[28ch] text-sm leading-6 text-muted-foreground text-pretty">{detail}</p>
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
    <Card className="border-border/70 bg-card/90">
      <CardHeader className="gap-4 border-b border-border/70 pb-5">
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-3">
            {eyebrow ? <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-primary">{eyebrow}</div> : null}
            <CardTitle className="font-display text-[2rem] leading-[1.02]">{title}</CardTitle>
            {description ? <CardDescription className="max-w-3xl text-base leading-7">{description}</CardDescription> : null}
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
    <label className="grid gap-3">
      <span className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">{label}</span>
      {children}
      {hint ? <span className="text-xs leading-6 text-muted-foreground text-pretty">{hint}</span> : null}
    </label>
  );
}

export function MetaLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4 border-t border-border/70 pt-4 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-semibold text-foreground">{value}</span>
    </div>
  );
}

export function DetailChip({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[1rem] border border-border/70 bg-secondary/75 px-4 py-4">
      <div className="text-[11px] font-semibold uppercase tracking-[0.2em] text-muted-foreground">{label}</div>
      <div className="mt-3 font-mono text-xl font-semibold tabular-nums text-foreground">{value}</div>
    </div>
  );
}
