import { PortalShell } from "../../../components/portal-shell";
import { formatStars, ratingColor } from "../../../lib/rating";

export const dynamic = "force-dynamic";

// Demo data
const leaderboard = [
  { providerId: "provider_1", name: "Atlas Ops", rating: 4.8, ratingCount: 23, totalOrders: 15, reputationTier: "gold" },
  { providerId: "provider_2", name: "Kite Relay", rating: 4.2, ratingCount: 11, totalOrders: 8, reputationTier: "silver" },
];

export default async function LeaderboardPage() {
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
        {leaderboard.map((entry, i) => {
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
                  <span className="px-2 py-0.5 bg-gray-100 rounded text-xs">{entry.reputationTier}</span>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </PortalShell>
  );
}
