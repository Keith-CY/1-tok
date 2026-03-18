export const leaderboardEntries = [
  {
    providerId: "provider_1",
    name: "Atlas Ops",
    rating: 4.8,
    ratingCount: 23,
    totalOrders: 15,
    reputationTier: "gold",
  },
  {
    providerId: "provider_2",
    name: "Kite Relay",
    rating: 4.2,
    ratingCount: 11,
    totalOrders: 8,
    reputationTier: "silver",
  },
  {
    providerId: "provider_3",
    name: "Cloudline Runtime",
    rating: 3.9,
    ratingCount: 9,
    totalOrders: 6,
    reputationTier: "bronze",
  },
] as const;

export const providerListingsCatalog = [
  {
    id: "lis_1",
    title: "Reliable agent triage agent",
    category: "agent-ops",
    tier: "gold",
    capacity: "40 tasks/day",
    punchline: "High-confidence remediation and escalation handling.",
  },
  {
    id: "lis_2",
    title: "Data pipeline helper",
    category: "data-pipeline",
    tier: "silver",
    capacity: "100 tasks/day",
    punchline: "Backfills, cleanup, and throughput recovery.",
  },
  {
    id: "lis_3",
    title: "Compute arbitrage runner",
    category: "compute",
    tier: "bronze",
    capacity: "60 tasks/day",
    punchline: "Burst capacity for runtime-heavy jobs.",
  },
] as const;

export const carrierJobs = [
  {
    id: "job_19",
    orderId: "ord_19",
    title: "Log ingestion remediation",
    status: "running",
    progress: "7/10",
    host: "carrier-runner-01",
    lastSeen: "2m ago",
  },
  {
    id: "job_20",
    orderId: "ord_20",
    title: "Pipeline burst scaling",
    status: "paused",
    progress: "3/6",
    host: "carrier-runner-07",
    lastSeen: "11m ago",
  },
  {
    id: "job_21",
    orderId: "ord_21",
    title: "Agent runtime audit",
    status: "pending",
    progress: "0/4",
    host: "carrier-runner-09",
    lastSeen: "28m ago",
  },
] as const;

export const opsApplications = [
  {
    id: "app_1",
    providerOrg: "Atlas Ops",
    category: "agent-ops",
    contact: "ops-support@atlas.io",
    status: "pending",
    submittedAt: "2026-03-15",
    notes: "Carrier-first remediation experience with SLA commitments.",
  },
  {
    id: "app_2",
    providerOrg: "Kite Relay",
    category: "agent-runtime",
    contact: "ops@kiterelay.io",
    status: "approved",
    submittedAt: "2026-03-12",
    notes: "Distributed execution and custom orchestration support.",
  },
  {
    id: "app_3",
    providerOrg: "Cloudline Runtime",
    category: "compute",
    contact: "infra@cloudline.run",
    status: "rejected",
    submittedAt: "2026-03-10",
    notes: "Duplicate identity; follow-up requested.",
  },
] as const;
