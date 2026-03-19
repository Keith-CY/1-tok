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
      <div className="grid flex-1 gap-14 lg:grid-cols-[0.92fr_1.08fr] lg:items-start">
        <section className="space-y-10 pt-3">
          <div className="space-y-7">
            <div className="eyebrow-pill">
              <RiPriceTag3Line className="size-3.5" />
              The digital broadsheet
            </div>
            <div className="space-y-5">
              <h1 className="max-w-4xl font-display text-6xl leading-[0.94] tracking-tight text-balance text-foreground sm:text-7xl">
                Price is visible. Delivery stays legible.
              </h1>
              <p className="max-w-2xl text-base leading-8 text-muted-foreground text-pretty">
                1-tok turns carrier, dispute, settlement, and delivery support into an editorial market surface. Clients publish a budget. Providers respond in public. Awards move directly into readable operational delivery.
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
              <Link href="/login?next=/provider">View the live request board</Link>
            </Button>
          </div>

          <div className="grid gap-6 border-t border-border/70 pt-6 sm:grid-cols-3">
            {marketStats.map((item) => (
              <div key={item.label} className="space-y-3">
                <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-muted-foreground">{item.label}</div>
                <div className="font-display text-4xl leading-none tracking-tight text-foreground">{item.value}</div>
                <div className="text-sm leading-7 text-muted-foreground">{item.detail}</div>
              </div>
            ))}
          </div>

          <div className="border-t border-border/70 pt-6">
            <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-primary">Working set</div>
            <div className="mt-4 flex flex-wrap gap-x-5 gap-y-3 text-sm text-muted-foreground">
              {marketExamples.map((item) => (
                <div key={item} className="inline-flex items-center gap-2">
                  <span className="text-primary">/</span>
                  <span>{item}</span>
                </div>
              ))}
            </div>
          </div>

          <div className="market-card px-6 py-6">
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
      <div className="inline-flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.22em] text-primary">
        <Icon className="size-4" />
        {title}
      </div>
      <p className="text-sm leading-7 text-muted-foreground">{detail}</p>
    </div>
  );
}
