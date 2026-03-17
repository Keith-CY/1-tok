import { PortalShell } from "../../../components/portal-shell";
import { EmptyState } from "../../../components/ui";
import { formatStars, ratingColor } from "../../../lib/rating";

export const dynamic = "force-dynamic";

// Demo data
const leaderboard = [
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
];

export default async function LeaderboardPage({
  searchParams,
}: {
  searchParams?: { q?: string; tier?: string; sort?: string };
}) {
  const query = (searchParams?.q ?? "").trim().toLowerCase();
  const tier = (searchParams?.tier ?? "all").toLowerCase();
  const sort = (searchParams?.sort ?? "rating").toLowerCase();

  const chipClass = (active: boolean) =>
    active ? "action-button action-button--active" : "action-button";

  const buildTierHref = (nextTier: string) => {
    const params = new URLSearchParams();

    if (query) {
      params.set("q", query);
    }

    if (nextTier !== "all") {
      params.set("tier", nextTier);
    }

    if (sort !== "rating") {
      params.set("sort", sort);
    }

    const queryString = params.toString();
    return queryString ? `/buyer/leaderboard?${queryString}` : "/buyer/leaderboard";
  };

  const buildSortHref = (nextSort: string) => {
    const params = new URLSearchParams();

    if (query) {
      params.set("q", query);
    }

    if (tier !== "all") {
      params.set("tier", tier);
    }

    if (nextSort !== "rating") {
      params.set("sort", nextSort);
    }

    const queryString = params.toString();
    return queryString ? `/buyer/leaderboard?${queryString}` : "/buyer/leaderboard";
  };

  const leaderboardData = leaderboard
    .filter(
      (entry) =>
        (tier === "all" || entry.reputationTier === tier) &&
        (!query || entry.name.toLowerCase().includes(query) || entry.providerId.toLowerCase().includes(query)),
    )
    .sort((a, b) => {
      if (sort === "orders") {
        return b.totalOrders - a.totalOrders;
      }

      if (sort === "reviews") {
        return b.ratingCount - a.ratingCount;
      }

      return b.rating - a.rating;
    });

  return (
    <PortalShell
      eyebrow="Marketplace"
      title="Provider Leaderboard"
      copy="Top-rated providers by marketplace performance."
      signal="Rankings"
      asideTitle="Quick info"
      quickActions={[
        { label: "Open listings", href: "/buyer/listings", tone: "primary" },
        { label: "Create RFQ", href: "/buyer/rfqs/create", tone: "secondary" },
      ]}
      asideItems={[]}
    >
      <div className="space-y-3">
        <div className="flex gap-2 mb-2">
          <a href={buildTierHref("all")} className={chipClass(tier === "all")} aria-current={tier === "all" ? "page" : undefined}>
            All tiers
          </a>
          <a href={buildTierHref("gold")} className={chipClass(tier === "gold")} aria-current={tier === "gold" ? "page" : undefined}>
            Gold
          </a>
          <a href={buildTierHref("silver")} className={chipClass(tier === "silver")} aria-current={tier === "silver" ? "page" : undefined}>
            Silver
          </a>
          <a href={buildTierHref("bronze")} className={chipClass(tier === "bronze")} aria-current={tier === "bronze" ? "page" : undefined}>
            Bronze
          </a>
        </div>

        <div className="flex gap-2 mb-2">
          <a href={buildSortHref("rating")} className={chipClass(sort === "rating")} aria-current={sort === "rating" ? "page" : undefined}>
            Sort by rating
          </a>
          <a href={buildSortHref("reviews")} className={chipClass(sort === "reviews")} aria-current={sort === "reviews" ? "page" : undefined}>
            Sort by reviews
          </a>
          <a href={buildSortHref("orders")} className={chipClass(sort === "orders")} aria-current={sort === "orders" ? "page" : undefined}>
            Sort by orders
          </a>
        </div>

        <form method="GET" className="auth-form market-form">
          <div className="market-form__grid">
            <label className="auth-field">
              <span>Search providers</span>
              <input
                name="q"
                type="text"
                placeholder="Search by name or provider id"
                defaultValue={query}
              />
            </label>
            <label className="auth-field">
              <span>Reputation tier</span>
              <select name="tier" defaultValue={tier}>
                <option value="all">All tiers</option>
                <option value="gold">Gold</option>
                <option value="silver">Silver</option>
                <option value="bronze">Bronze</option>
              </select>
            </label>
            <label className="auth-field">
              <span>Sort by</span>
              <select name="sort" defaultValue={sort}>
                <option value="rating">Rating</option>
                <option value="reviews">Review count</option>
                <option value="orders">Orders</option>
              </select>
            </label>
          </div>
          <button type="submit" className="auth-submit">
            Filter leaderboard
          </button>
        </form>

        {leaderboardData.length === 0 ? (
          <EmptyState
            icon="🏆"
            message="No providers match your leaderboard filters."
            actionLabel="Clear filters"
            actionHref="/buyer/leaderboard"
          />
        ) : (
          <div className="space-y-3">
            {leaderboardData.map((entry, i) => {
              const medal = i === 0 ? "🥇" : i === 1 ? "🥈" : i === 2 ? "🥉" : `#${i + 1}`;
              return (
                <div key={entry.providerId} className="border rounded-lg p-4 flex items-center gap-4">
                  <span className="text-2xl w-10 text-center">{medal}</span>
                  <div className="flex-1">
                    <h3 className="font-semibold text-lg">{entry.name}</h3>
                    <div className="flex items-center gap-3 text-sm text-gray-500">
                      <span className={ratingColor(entry.rating)}>
                        {formatStars(entry.rating)} {entry.rating.toFixed(1)}
                      </span>
                      <span>({entry.ratingCount} reviews)</span>
                      <span>{entry.totalOrders} orders</span>
                      <span className="bg-gray-100 text-gray-700 px-2 py-0.5 rounded text-xs">{entry.reputationTier}</span>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </PortalShell>
  );
}
