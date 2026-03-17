import { PortalShell } from "../../../components/portal-shell";
import { EmptyState } from "../../../components/ui";
import { searchListings } from "../../../lib/api";

export const dynamic = "force-dynamic";

export default async function BuyerListingsPage({
  searchParams,
}: {
  searchParams: { q?: string; category?: string; tag?: string; sort?: string };
}) {
  const params = searchParams;
  const q = (params?.q ?? "").trim().toLowerCase();
  const category = (params?.category ?? "").trim();
  const tag = (params?.tag ?? "").trim();
  const sort = (params?.sort ?? "category").trim().toLowerCase() || "category";
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

  const buildSortHref = (nextSort: string) => {
    const params = new URLSearchParams();

    if (q) {
      params.set("q", q);
    }

    if (category) {
      params.set("category", category);
    }

    if (tag) {
      params.set("tag", tag);
    }

    if (nextSort !== "category") {
      params.set("sort", nextSort);
    }

    const queryString = params.toString();
    return queryString ? `/buyer/listings?${queryString}` : "/buyer/listings";
  };

  const buildCategoryHref = (nextCategory: string) => {
    const params = new URLSearchParams();

    if (q) {
      params.set("q", q);
    }

    if (tag) {
      params.set("tag", tag);
    }

    if (nextCategory) {
      params.set("category", nextCategory);
    }

    const queryString = params.toString();
    return queryString ? `/buyer/listings?${queryString}` : "/buyer/listings";
  };

  const chipClass = (active: boolean) =>
    active ? "action-button action-button--active" : "action-button";

  const sortedListings = [...listings].sort((a, b) => {
    if (sort === "price-asc") {
      return (a.basePriceCents ?? 0) - (b.basePriceCents ?? 0);
    }

    if (sort === "price-desc") {
      return (b.basePriceCents ?? 0) - (a.basePriceCents ?? 0);
    }

    return String(a.category || "").localeCompare(String(b.category || ""));
  });

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
        <div className="flex gap-2 mb-2 flex-wrap">
          <a href={buildCategoryHref("")} className={chipClass(category === "")} aria-current={category === "" ? "page" : undefined}>
            All categories
          </a>
          <a href={buildCategoryHref("agent-ops")} className={chipClass(category === "agent-ops")} aria-current={category === "agent-ops" ? "page" : undefined}>
            Agent Ops
          </a>
          <a href={buildCategoryHref("agent-runtime")} className={chipClass(category === "agent-runtime")} aria-current={category === "agent-runtime" ? "page" : undefined}>
            Agent Runtime
          </a>
          <a href={buildCategoryHref("compute")} className={chipClass(category === "compute")} aria-current={category === "compute" ? "page" : undefined}>
            Compute
          </a>
        </div>
        <div className="flex gap-2 mb-2">
          <a href={buildSortHref("category")} className={chipClass(sort === "category")} aria-current={sort === "category" ? "page" : undefined}>
            Sort: Category
          </a>
          <a href={buildSortHref("price-asc")} className={chipClass(sort === "price-asc")} aria-current={sort === "price-asc" ? "page" : undefined}>
            Price: Low to high
          </a>
          <a href={buildSortHref("price-desc")} className={chipClass(sort === "price-desc")} aria-current={sort === "price-desc" ? "page" : undefined}>
            Price: High to low
          </a>
        </div>
        <form method="GET" className="auth-form market-form">
          <input type="hidden" name="sort" value={sort} />
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

        {sortedListings.length === 0 ? (
          <EmptyState
            message="No listings found. Try adjusting your search."
            actionLabel="Clear filters"
            actionHref="/buyer/listings"
          />
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            {sortedListings.map((listing: any) => (
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
