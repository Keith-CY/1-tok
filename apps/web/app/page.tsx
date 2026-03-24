import Link from "next/link";

import { Button } from "@/components/ui/button";

export const dynamic = "force-dynamic";

const proofMarks = ["CKB settlement", "Carrier-backed delivery", "Milestone payouts", "Open request board", "Provider bidding", "Ops review"] as const;

const featureColumns = [
  {
    eyebrow: "Public market",
    title: "Budgets stay public.",
    copy:
      "Clients do not start from blind outreach. They publish a scoped request, a real budget, and a deadline. Providers price against the same sheet.",
  },
  {
    eyebrow: "Controlled settlement",
    title: "Settlement stays traceable.",
    copy:
      "1-tok turns deposit, reserve, milestone payout, and dispute posture into one readable operating surface instead of four separate tools.",
  },
  {
    eyebrow: "Execution handoff",
    title: "Carrier plugs into the board.",
    copy:
      "Providers bring their execution stack. Buyers still get one board for request, award, delivery, and completion without losing payout discipline.",
  },
] as const;

const roleLanes = [
  {
    role: "Buyer",
    title: "Post, fund, award.",
    copy:
      "A buyer gets one fixed USDI deposit address, one request board, and one delivery lane. Budget, current low quote, and completion posture stay readable at a glance.",
    href: "/login?next=/buyer",
    action: "Enter buyer workspace",
  },
  {
    role: "Provider",
    title: "Bid with the board in view.",
    copy:
      "Providers read the same live board buyers do, submit a price, and move into delivery only after award and settlement rail checks are in place.",
    href: "/login?next=/provider",
    action: "Enter provider workspace",
  },
  {
    role: "Ops",
    title: "Run treasury from one desk.",
    copy:
      "Operations does not need a separate finance console and workflow console. Demo readiness, treasury posture, settlement health, and disputes share one quiet view.",
    href: "/internal/login",
    action: "Enter ops workspace",
  },
] as const;

const marketSignals = [
  { label: "Highest budget", value: "$780" },
  { label: "Current low proposal", value: "$320" },
  { label: "Average time to award", value: "2.4 hr" },
] as const;

const previewSheets = [
  {
    eyebrow: "Request sheet",
    title: "What the buyer sees first",
    metrics: [
      { label: "Budget", value: "$640" },
      { label: "Low quote", value: "$480" },
      { label: "Deadline", value: "2 hrs" },
    ],
    note: "One request, one budget, one live low quote.",
  },
  {
    eyebrow: "Deposit sheet",
    title: "What funding looks like",
    metrics: [
      { label: "Asset", value: "USDI" },
      { label: "Threshold", value: "10.00" },
      { label: "Confirm", value: "24 blocks" },
    ],
    note: "A fixed buyer address turns confirmed chain balance into credited USD.",
  },
  {
    eyebrow: "Delivery sheet",
    title: "What ops keeps readable",
    metrics: [
      { label: "Reserve", value: "Live" },
      { label: "Payout", value: "Ready" },
      { label: "Dispute", value: "72 hrs" },
    ],
    note: "Award, payout, and review stay attached to one visible treasury trail.",
  },
] as const;

const proofFlow = [
  {
    number: "01",
    label: "Deposit",
    title: "Fund from one fixed address.",
    copy: "Each buyer keeps a dedicated CKB address for USDI credit instead of starting from a manual billing loop.",
  },
  {
    number: "02",
    label: "Award",
    title: "Open delivery only after checks clear.",
    copy: "The board can show price, budget, and funding posture before the client moves a provider into live work.",
  },
  {
    number: "03",
    label: "Payout",
    title: "Settle against visible evidence.",
    copy: "Milestones release from treasury through a payout trail that stays inspectable by ops and legible to the buyer.",
  },
] as const;

export default function HomePage() {
  return (
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto max-w-[96rem] px-5 py-6 sm:px-8 lg:px-12">
        <header className="flex flex-col gap-6 border-b border-black/8 pb-8 lg:flex-row lg:items-center lg:justify-between">
          <div className="space-y-2">
            <div className="eyebrow-pill">1-tok</div>
            <div className="text-sm leading-7 text-muted-foreground">A market board for scoped expert work, controlled settlement, and carrier-backed delivery.</div>
          </div>
          <div className="flex flex-wrap items-center gap-5 text-sm text-muted-foreground">
            <Link href="#product" className="border-b border-transparent pb-1 transition-colors hover:border-black/20 hover:text-foreground">
              Product
            </Link>
            <Link href="#roles" className="border-b border-transparent pb-1 transition-colors hover:border-black/20 hover:text-foreground">
              Roles
            </Link>
            <Link href="/login" className="border-b border-transparent pb-1 transition-colors hover:border-black/20 hover:text-foreground">
              Login
            </Link>
            <Button asChild size="sm">
              <Link href="/login?next=/buyer">Start with a request</Link>
            </Button>
          </div>
        </header>

        <section className="grid gap-10 py-12 xl:grid-cols-[1.06fr_0.94fr] xl:items-start xl:py-20">
          <div className="space-y-10">
            <div className="space-y-6">
              <div className="eyebrow-pill">Open request market / USDI-funded / carrier-backed</div>
              <div className="space-y-5">
                <h1 className="max-w-5xl font-display text-[clamp(3.5rem,7vw,7rem)] font-medium leading-[0.92] tracking-[-0.04em] text-balance">
                  Price wins the work.
                </h1>
                <p className="max-w-3xl text-lg leading-9 text-muted-foreground text-pretty">
                  1-tok gives a client one board for scoped requests, visible budgets, provider quotes, on-chain funding, and milestone payout posture.
                </p>
              </div>
            </div>

            <div className="flex flex-wrap gap-3">
              <Button asChild size="lg">
                <Link href="/login?next=/buyer">Post a request</Link>
              </Button>
              <Button asChild variant="outline" size="lg">
                <Link href="/login?next=/provider">View provider lane</Link>
              </Button>
            </div>

            <div className="grid gap-3 sm:grid-cols-3">
              {marketSignals.map((signal) => (
                <div key={signal.label} className="bg-secondary px-5 py-5">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">{signal.label}</div>
                  <div className="mt-3 font-mono text-3xl font-semibold tracking-tight tabular-nums text-foreground">{signal.value}</div>
                </div>
              ))}
            </div>
          </div>

          <div className="space-y-4">
            <div className="sheet-stack px-5 py-5 sm:px-7 sm:py-7">
              <div className="space-y-2">
                <div className="eyebrow-pill">Board preview</div>
                <div className="font-display text-3xl font-medium leading-[1.04] tracking-[-0.03em]">What stays visible on day one.</div>
              </div>

              <div className="mt-8 space-y-3">
                {previewSheets.map((sheet) => (
                  <div key={sheet.title} className="bg-card px-5 py-5 shadow-[0_20px_40px_rgba(0,0,0,0.06)]">
                    <div className="space-y-4">
                      <div className="space-y-2">
                        <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-accent">{sheet.eyebrow}</div>
                        <div className="font-display text-2xl font-medium leading-[1.06] tracking-[-0.02em] text-balance">{sheet.title}</div>
                      </div>
                      <div className="grid gap-4 sm:grid-cols-3">
                        {sheet.metrics.map((metric) => (
                          <LandingMetric key={`${sheet.title}-${metric.label}`} label={metric.label} value={metric.value} />
                        ))}
                      </div>
                      <p className="max-w-2xl text-sm leading-7 text-muted-foreground">{sheet.note}</p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        <section className="grid gap-4 pb-10 xl:grid-cols-3 xl:gap-5 xl:pb-14">
          {proofFlow.map((item) => (
            <div key={item.label} className="bg-card px-6 py-6 shadow-[0_20px_40px_rgba(0,0,0,0.06)] sm:px-7 sm:py-7">
              <div className="space-y-5">
                <div className="flex items-start justify-between gap-4">
                  <div className="eyebrow-pill">{item.label}</div>
                  <div className="font-mono text-sm text-muted-foreground">{item.number}</div>
                </div>
                <div className="space-y-3">
                  <div className="font-display text-[clamp(2rem,2.6vw,2.7rem)] font-medium leading-[1.02] tracking-[-0.03em] text-balance">
                    {item.title}
                  </div>
                  <p className="max-w-2xl text-base leading-8 text-muted-foreground text-pretty">{item.copy}</p>
                </div>
              </div>
            </div>
          ))}
        </section>

        <section className="bg-secondary px-5 py-6 sm:px-7" aria-label="Proof strip">
          <div className="space-y-5">
            <div className="max-w-3xl text-sm leading-7 text-muted-foreground">
              1-tok is designed for teams who need request publishing, controlled settlement, and delivery oversight to read like one system instead of three stitched dashboards.
            </div>
            <div className="flex flex-wrap gap-x-6 gap-y-3 text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">
              {proofMarks.map((mark) => (
                <span key={mark}>{mark}</span>
              ))}
            </div>
          </div>
        </section>

        <section id="product" className="grid gap-5 py-16 xl:grid-cols-3">
          {featureColumns.map((feature) => (
            <div key={feature.title} className="sheet-stack px-6 py-6 sm:px-7 sm:py-7">
              <div className="space-y-4">
                <div className="eyebrow-pill">{feature.eyebrow}</div>
                <h2 className="font-display text-3xl font-medium leading-[1.08] tracking-[-0.03em] text-balance">{feature.title}</h2>
                <p className="text-base leading-8 text-muted-foreground text-pretty">{feature.copy}</p>
              </div>
            </div>
          ))}
        </section>

        <section id="roles" className="grid gap-5 pb-16 xl:grid-cols-3">
          {roleLanes.map((lane) => (
            <div key={lane.role} className="bg-card px-6 py-6 shadow-[0_20px_40px_rgba(0,0,0,0.06)]">
              <div className="space-y-5">
                <div className="eyebrow-pill">{lane.role}</div>
                <div className="space-y-3">
                  <h2 className="font-display text-3xl font-medium leading-[1.08] tracking-[-0.03em] text-balance">{lane.title}</h2>
                  <p className="text-base leading-8 text-muted-foreground text-pretty">{lane.copy}</p>
                </div>
                <Button asChild variant="outline">
                  <Link href={lane.href}>{lane.action}</Link>
                </Button>
              </div>
            </div>
          ))}
        </section>

        <section className="sheet-stack mb-8 px-6 py-8 sm:px-8 sm:py-10">
          <div className="grid gap-8 xl:grid-cols-[1.2fr_0.8fr] xl:items-end">
            <div className="space-y-4">
              <div className="eyebrow-pill">Start from the request, not from the workflow sprawl</div>
              <h2 className="max-w-4xl font-display text-[clamp(2.6rem,5vw,4.8rem)] font-medium leading-[0.96] tracking-[-0.04em] text-balance">
                Quiet board. Hard evidence.
              </h2>
              <p className="max-w-3xl text-base leading-8 text-muted-foreground text-pretty">
                Use the buyer lane to post a scoped request, the provider lane to watch live price pressure, and the ops lane to inspect readiness, treasury posture, and dispute flow without changing systems.
              </p>
            </div>
            <div className="flex flex-wrap gap-3 xl:justify-end">
              <Button asChild size="lg">
                <Link href="/login?next=/buyer">Open buyer lane</Link>
              </Button>
              <Button asChild variant="outline" size="lg">
                <Link href="/login?next=/provider">Open provider lane</Link>
              </Button>
            </div>
          </div>
        </section>
      </div>
    </main>
  );
}

function LandingMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="space-y-2">
      <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">{label}</div>
      <div className="font-mono text-2xl font-semibold tracking-tight tabular-nums text-foreground">{value}</div>
    </div>
  );
}
