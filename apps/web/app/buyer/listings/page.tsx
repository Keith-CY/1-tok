import { PortalShell } from "../../../components/portal-shell";
import { EmptyState } from "../../../components/ui";
import { searchListings } from "../../../lib/api";

export const dynamic = "force-dynamic";

export default async function BuyerListingsPage({
  searchParams,
}: {
  searchParams: { q?: string; category?: string; tag?: string };
}) {
  const params = searchParams;
  const q = (params?.q ?? "").trim();
  const category = (params?.category ?? "").trim();
  const tag = (params?.tag ?? "").trim();
  let listings: any[] = [];
  let error = "";

  try {
    const result = await searchListings({
      q,
      category,
      tag,
    });
    listings = result.listings;
  } catch (e: any) {
    error = e.message;
  }

  return (
    <PortalShell
      eyebrow="Buyer portal / discover"
      title="Find the right agent provider."
      copy="Search by capability, category, or price range. Compare providers and start an RFQ."
      signal="Listing discovery"
      asideTitle="Quick info"
      quickActions={[
        { label: "Back to buyer dashboard", href: "/buyer", tone: "secondary" },
        { label: "Search RFQs", href: "/buyer/rfqs", tone: "primary" },
      ]}
      asideItems={[]}
    >
      <div className="space-y-4">
        <form method="GET" className="auth-form market-form">
          <div className="market-form__grid">
            <label className="auth-field">
              <span>Search keyword</span>
              <input
                name="q"
                type="text"
                placeholder="Search listings..."
                defaultValue={q}
              />
            </label>
            <label className="auth-field">
              <span>Category</span>
              <select name="category" defaultValue={category}>
                <option value="">All categories</option>
                <option value="agent-ops">Agent Ops</option>
                <option value="agent-runtime">Agent Runtime</option>
                <option value="compute">Compute</option>
              </select>
            </label>
            <label className="auth-field">
              <span>Tag</span>
              <input
                name="tag"
                type="text"
                placeholder="Search by tag"
                defaultValue={tag}
              />
            </label>
          </div>
          <button type="submit" className="auth-submit">
            Search
          </button>
        </form>

        {error && <p className="text-red-500">{error}</p>}

        {listings.length === 0 ? (
          <EmptyState
            message="No listings found. Try adjusting your search."
            actionLabel="Clear filters"
            actionHref="/buyer/listings"
          />
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            {listings.map((listing: any) => (
              <div key={listing.id} className="border rounded-lg p-4 hover:shadow-md transition-shadow">
                <h3 className="font-semibold text-lg">{listing.title}</h3>
                <p className="text-sm text-gray-500">{listing.category}</p>
                <p className="text-lg font-bold mt-2">${(listing.basePriceCents / 100).toFixed(2)}</p>
                {listing.tags?.length > 0 && (
                  <div className="flex gap-1 mt-2">
                    {listing.tags.map((tag: string) => (
                      <span key={tag} className="bg-gray-100 text-gray-700 text-xs px-2 py-1 rounded">
                        {tag}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </PortalShell>
  );
}
