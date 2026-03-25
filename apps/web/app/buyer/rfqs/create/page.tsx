import Link from "next/link";
import { RiArrowLeftLine, RiAuctionLine, RiFlashlightLine, RiPriceTag3Line, RiSearchLine, RiTimeLine } from "react-icons/ri";

import { CreateRFQWorkspace } from "@/components/create-rfq-workspace";
import { WorkspaceShell } from "@/components/workspace-shell";

export const metadata = {
  title: "New Request",
};

export const dynamic = "force-dynamic";

export default async function CreateRFQPage() {
  return (
    <WorkspaceShell
      role="buyer"
      title="Post a priced request"
      description="For the demo, post a short research request with a clear budget and delivery window."
      actions={[{ href: "/buyer", label: "Back to requests", icon: RiArrowLeftLine, variant: "outline" }]}
    >
      <section className="sheet-stack overflow-hidden">
        <div className="grid gap-px bg-border lg:grid-cols-[1fr_1fr_1fr_1fr]">
          <MetricStrip
            icon={RiFlashlightLine}
            label="Listing mode"
            value="Open market"
            detail="Providers see the request as soon as it is posted."
          />
          <MetricStrip
            icon={RiPriceTag3Line}
            label="Budget"
            value="Required"
            detail="A clear budget pulls real pricing into the request."
          />
          <MetricStrip
            icon={RiTimeLine}
            label="Delivery window"
            value="Required"
            detail="Timing changes price, so it has to be explicit."
          />
          <MetricStrip
            icon={RiAuctionLine}
            label="Award path"
            value="Low wins"
            detail="You compare proposals on price and timing before award."
          />
        </div>
      </section>

      <section className="mt-6 rounded-md bg-[var(--surface-lowest)] px-6 py-5 shadow-[0_20px_40px_rgba(0,0,0,0.06)] sm:px-7">
        <div className="space-y-2">
          <div className="inline-flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.22em] text-accent">
            <RiSearchLine className="size-4" />
            Demo note
          </div>
          <p className="text-sm leading-7 text-foreground">
            This demo works best with research requests such as vendor scans, market comparisons, pricing checks, or short recommendation memos. Avoid operational or fulfillment-heavy requests in the demo flow.
          </p>
        </div>
      </section>

      <CreateRFQWorkspace />
    </WorkspaceShell>
  );
}

function MetricStrip({
  icon: Icon,
  label,
  value,
  detail,
}: {
  icon: typeof RiFlashlightLine;
  label: string;
  value: string;
  detail: string;
}) {
  return (
    <div className="flex min-h-[11.5rem] flex-col justify-between bg-[var(--surface-lowest)] px-5 py-6">
      <div className="space-y-3">
        <div className="inline-flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
          <Icon className="size-4 text-accent" />
          {label}
        </div>
        <div className="max-w-[12rem] font-display text-[2rem] font-medium leading-[1.02] tracking-[-0.03em] text-foreground">
          {value}
        </div>
      </div>
      <div className="max-w-[15rem] text-sm leading-7 text-muted-foreground">{detail}</div>
    </div>
  );
}
