import Link from "next/link";

import { PortalShell } from "../../../components/portal-shell";
import { EmptyState } from "../../../components/ui";
import { requirePortalViewer } from "../../../lib/viewer";

export const dynamic = "force-dynamic";

const MOCK_LISTINGS = [
  {
    id: "lis_1",
    title: "Reliable agent triage agent",
    category: "agent-ops",
    tier: "gold",
    capacity: "40 tasks/day",
  },
  {
    id: "lis_2",
    title: "Data pipeline helper",
    category: "data-pipeline",
    tier: "silver",
    capacity: "100 tasks/day",
  },
  {
    id: "lis_3",
    title: "Compute arbitrage runner",
    category: "compute",
    tier: "bronze",
    capacity: "60 tasks/day",
  },
];

export default async function ProviderListingsPage({
  searchParams,
}: {
  searchParams?: { q?: string; category?: string };
}) {
  const viewer = await requirePortalViewer("provider", "/provider/listings");

  const q = (searchParams?.q ?? "").trim().toLowerCase();
  const selectedCategory = (searchParams?.category ?? "all").toLowerCase();

  const filteredListings = MOCK_LISTINGS.filter(
    (item) =>
      (!q || item.title.toLowerCase().includes(q) || item.category.toLowerCase().includes(q)) &&
      (selectedCategory === "all" || item.category === selectedCategory),
  );

  return (
    <PortalShell
      eyebrow="Provider portal / listings"
      title="Manage your listings."
      copy="Create and edit listings to showcase your agent runtime capabilities."
      signal="Provider listings"
      asideTitle="Quick info"
      quickActions={[
        { label: "Create first listing", href: "/provider/listings/create", tone: "primary" },
        { label: "Return to provider dashboard", href: "/provider", tone: "secondary" },
      ]}
      asideItems={[]}
    >
      <div className="space-y-4">
        <div className="flex justify-between items-center">
          <h2 className="text-lg font-semibold">Your Listings</h2>
          <Link href="/provider/listings/create" className="action-button">
            + New Listing
          </Link>
        </div>

        <form method="GET" className="auth-form market-form">
          <div className="market-form__grid">
            <label className="auth-field">
              <span>Search listings</span>
              <input
                name="q"
                type="text"
                placeholder="Search by title or category"
                defaultValue={searchParams?.q ?? ""}
              />
            </label>
            <label className="auth-field">
              <span>Category</span>
              <select name="category" defaultValue={searchParams?.category ?? "all"}>
                <option value="all">All categories</option>
                <option value="agent-ops">Agent Ops</option>
                <option value="agent-runtime">Agent Runtime</option>
                <option value="compute">Compute</option>
                <option value="data-pipeline">Data Pipeline</option>
              </select>
            </label>
          </div>
          <button type="submit" className="auth-submit">
            Find listings
          </button>
        </form>

        {filteredListings.length === 0 ? (
          <EmptyState
            message="No listings match your filters."
            actionLabel="Clear filters"
            actionHref="/provider/listings"
          />
        ) : (
          <div className="card-grid">
            {filteredListings.map((listing) => (
              <article key={listing.id} className="glass-card">
                <div className="space-y-2">
                  <p className="text-sm text-muted">{listing.tier}</p>
                  <h3 className="text-lg font-semibold">{listing.title}</h3>
                  <p className="text-sm text-gray-500">Category: {listing.category}</p>
                  <p className="text-sm text-gray-500">Capacity: {listing.capacity}</p>
                  <div className="flex gap-2 pt-3">
                    <a href="/provider/listings/create" className="action-button">
                      Edit pricing
                    </a>
                    <Link href="/provider/rfqs" className="action-button">
                      View RFQ matches
                    </Link>
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </div>
    </PortalShell>
  );
}
