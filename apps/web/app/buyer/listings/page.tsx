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
  let listings: any[] = [];
  let error = "";

  try {
    const result = await searchListings({
      q: params.q,
      category: params.category,
      tag: params.tag,
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
      asideItems={[]}
    >
      <div className="space-y-4">
        <form method="GET" className="flex gap-2">
          <input
            name="q"
            type="text"
            placeholder="Search listings..."
            defaultValue={params.q ?? ""}
            className="border rounded px-3 py-2 flex-1"
          />
          <select name="category" defaultValue={params.category ?? ""} className="border rounded px-3 py-2">
            <option value="">All categories</option>
            <option value="agent-ops">Agent Ops</option>
            <option value="agent-runtime">Agent Runtime</option>
            <option value="compute">Compute</option>
          </select>
          <button type="submit" className="bg-blue-600 text-white px-4 py-2 rounded">
            Search
          </button>
        </form>

        {error && <p className="text-red-500">{error}</p>}

        {listings.length === 0 ? (
          <EmptyState
            message="No listings found. Try adjusting your search."
            actionLabel="Create an RFQ"
            actionHref="/buyer"
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
