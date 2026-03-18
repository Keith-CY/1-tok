import Link from "next/link";
import { RiArrowRightUpLine, RiAuctionLine, RiPriceTag3Line, RiTimeLine } from "react-icons/ri";

import { PublicShell } from "@/components/workspace-shell";
import { RoleShowcase } from "@/components/role-showcase";
import { Button } from "@/components/ui/button";

export const dynamic = "force-dynamic";

const marketStats = [
  { label: "Top live budget", value: "$8.4k", detail: "Carrier dispute response package" },
  { label: "Current live floor", value: "$3.9k", detail: "Carrier onboarding pack" },
  { label: "Requests closing today", value: "14", detail: "Across active client listings" },
] as const;

const marketExamples = [
  "Carrier onboarding",
  "Dispute response",
  "Milestone delivery support",
  "Settlement reconciliation",
] as const;

export default function HomePage() {
  return (
    <PublicShell>
      <div className="grid flex-1 gap-10 lg:grid-cols-[0.9fr_1.1fr] lg:items-start">
        <section className="space-y-8 pt-2">
          <div className="space-y-6">
            <div className="eyebrow-pill">
              <RiPriceTag3Line className="size-3.5" />
              1-tok marketplace
            </div>
            <div className="space-y-4">
              <h1 className="max-w-4xl text-5xl font-semibold leading-[1.02] tracking-tight text-balance sm:text-6xl">
                Price wins the work.
              </h1>
              <p className="max-w-2xl text-base leading-8 text-muted-foreground text-pretty">
                1-tok turns carrier, dispute, settlement, and delivery support into open requests. Clients publish a budget. Providers answer with live proposals and delivery windows.
              </p>
            </div>
          </div>

          <div className="flex flex-wrap gap-3">
            <Button asChild size="lg">
              <Link href="/login?next=/buyer">
                Post a request
                <RiArrowRightUpLine className="size-4" />
              </Link>
            </Button>
            <Button asChild variant="outline" size="lg">
              <Link href="/login?next=/provider">View live requests</Link>
            </Button>
          </div>

          <div className="rounded-md border border-border bg-card">
            <div className="grid gap-px rounded-md bg-border sm:grid-cols-3">
              {marketStats.map((item) => (
                <div key={item.label} className="space-y-2 bg-card px-5 py-5">
                  <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{item.label}</div>
                  <div className="font-mono text-3xl font-semibold tracking-tight tabular-nums text-foreground">{item.value}</div>
                  <div className="text-sm text-muted-foreground">{item.detail}</div>
                </div>
              ))}
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            {marketExamples.map((item) => (
              <div key={item} className="rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground shadow-[0_8px_18px_-16px_rgba(15,23,42,0.18)]">
                {item}
              </div>
            ))}
          </div>

          <div className="rounded-md border border-border bg-card px-5 py-4">
            <div className="grid gap-4 sm:grid-cols-3">
              <InfoColumn
                icon={RiAuctionLine}
                title="Budget is visible"
                detail="Providers can see immediately whether the request is worth pricing."
              />
              <InfoColumn
                icon={RiPriceTag3Line}
                title="Proposals stay comparable"
                detail="The market keeps price and delivery window in the same decision view."
              />
              <InfoColumn
                icon={RiTimeLine}
                title="Timing stays explicit"
                detail="Shorter delivery windows push harder pricing pressure across the board."
              />
            </div>
          </div>
        </section>

        <RoleShowcase />
      </div>
    </PublicShell>
  );
}

function InfoColumn({
  icon: Icon,
  title,
  detail,
}: {
  icon: typeof RiAuctionLine;
  title: string;
  detail: string;
}) {
  return (
    <div className="min-w-0 space-y-2">
      <div className="inline-flex items-center gap-2 text-xs font-medium text-primary">
        <Icon className="size-4" />
        {title}
      </div>
      <p className="text-sm leading-7 text-muted-foreground">{detail}</p>
    </div>
  );
}
