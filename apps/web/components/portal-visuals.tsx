import type { ReactNode, SVGProps } from "react";

export type PortalTheme = "buyer" | "provider" | "ops" | "login";
export type GlyphName =
  | "buyer"
  | "provider"
  | "ops"
  | "login"
  | "rfq"
  | "orders"
  | "bids"
  | "payout"
  | "treasury"
  | "signal"
  | "message"
  | "listing"
  | "search"
  | "dispute"
  | "info";

const glyphPaths: Record<GlyphName, ReactNode> = {
  buyer: (
    <>
      <rect x="5" y="7" width="14" height="10" rx="3" />
      <path d="M8 10h8" />
      <path d="M10 14h4" />
    </>
  ),
  provider: (
    <>
      <path d="M6 15V9l6-4 6 4v6l-6 4-6-4Z" />
      <path d="M12 8v8" />
      <path d="M9 11h6" />
    </>
  ),
  ops: (
    <>
      <path d="M12 4 6 7v5c0 4 2.5 6.3 6 8 3.5-1.7 6-4 6-8V7l-6-3Z" />
      <path d="m9.5 12 1.7 1.8 3.3-3.6" />
    </>
  ),
  login: (
    <>
      <rect x="6" y="4" width="9" height="16" rx="2.5" />
      <path d="M15 12h4" />
      <path d="m17 10 2 2-2 2" />
    </>
  ),
  rfq: (
    <>
      <path d="M8 4h6l4 4v12H8z" />
      <path d="M14 4v4h4" />
      <path d="M10 12h6" />
      <path d="M10 16h4" />
    </>
  ),
  orders: (
    <>
      <rect x="4" y="6" width="16" height="12" rx="3" />
      <path d="M8 10h8" />
      <path d="M8 14h5" />
    </>
  ),
  bids: (
    <>
      <path d="M6 18V9" />
      <path d="M12 18V5" />
      <path d="M18 18v-7" />
      <path d="M4 18h16" />
    </>
  ),
  payout: (
    <>
      <circle cx="12" cy="12" r="7" />
      <path d="M9 10h6" />
      <path d="M9 14h5" />
    </>
  ),
  treasury: (
    <>
      <rect x="4" y="6" width="16" height="12" rx="3" />
      <circle cx="12" cy="12" r="2.5" />
      <path d="M4 10h16" />
    </>
  ),
  signal: (
    <>
      <path d="M4 15c2.5 0 2.5-6 5-6s2.5 6 5 6 2.5-6 5-6" />
      <path d="M4 19h16" />
    </>
  ),
  message: (
    <>
      <path d="M5 7.5a2.5 2.5 0 0 1 2.5-2.5h9A2.5 2.5 0 0 1 19 7.5v6a2.5 2.5 0 0 1-2.5 2.5H10l-4 3v-3H7.5A2.5 2.5 0 0 1 5 13.5z" />
    </>
  ),
  listing: (
    <>
      <rect x="4" y="5" width="7" height="7" rx="1.5" />
      <rect x="13" y="5" width="7" height="7" rx="1.5" />
      <rect x="4" y="14" width="7" height="7" rx="1.5" />
      <rect x="13" y="14" width="7" height="7" rx="1.5" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="5.5" />
      <path d="m15.5 15.5 3.5 3.5" />
    </>
  ),
  dispute: (
    <>
      <path d="M12 5 4.5 18h15Z" />
      <path d="M12 9v4" />
      <path d="M12 16h.01" />
    </>
  ),
  info: (
    <>
      <circle cx="12" cy="12" r="8" />
      <path d="M12 10v5" />
      <path d="M12 7.5h.01" />
    </>
  ),
};

export function Glyph({
  name,
  className,
  ...props
}: SVGProps<SVGSVGElement> & {
  name: GlyphName;
}) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={className}
      {...props}
    >
      {glyphPaths[name]}
    </svg>
  );
}

export function resolvePortalTheme(eyebrow: string): PortalTheme {
  const normalized = eyebrow.toLowerCase();

  if (normalized.startsWith("buyer")) return "buyer";
  if (normalized.startsWith("provider")) return "provider";
  if (normalized.startsWith("ops")) return "ops";
  return "login";
}

export function resolveGlyphFromText(text: string): GlyphName {
  const normalized = text.toLowerCase();

  if (normalized.includes("buyer")) return "buyer";
  if (normalized.includes("provider")) return "provider";
  if (normalized.includes("ops")) return "ops";
  if (normalized.includes("login") || normalized.includes("session")) return "login";
  if (normalized.includes("rfq") || normalized.includes("quote")) return "rfq";
  if (normalized.includes("order")) return "orders";
  if (normalized.includes("bid")) return "bids";
  if (normalized.includes("payout") || normalized.includes("withdraw")) return "payout";
  if (normalized.includes("treasury") || normalized.includes("invoice")) return "treasury";
  if (normalized.includes("inbox") || normalized.includes("message")) return "message";
  if (normalized.includes("listing") || normalized.includes("catalog")) return "listing";
  if (normalized.includes("search") || normalized.includes("filter")) return "search";
  if (normalized.includes("dispute") || normalized.includes("risk")) return "dispute";
  return "signal";
}

const posterConfig: Record<
  PortalTheme,
  { eyebrow: string; title: string; glyph: GlyphName; scenes: Array<{ label: string; glyph: GlyphName }> }
> = {
  buyer: {
    eyebrow: "Buyer board",
    title: "Demand / budget / award",
    glyph: "buyer",
    scenes: [
      { label: "RFQ", glyph: "rfq" },
      { label: "Bids", glyph: "bids" },
      { label: "Orders", glyph: "orders" },
    ],
  },
  provider: {
    eyebrow: "Provider board",
    title: "Opportunities / quote / payout",
    glyph: "provider",
    scenes: [
      { label: "Queue", glyph: "bids" },
      { label: "Payout", glyph: "payout" },
      { label: "Catalog", glyph: "listing" },
    ],
  },
  ops: {
    eyebrow: "Ops board",
    title: "Credit / disputes / treasury",
    glyph: "ops",
    scenes: [
      { label: "Credit", glyph: "signal" },
      { label: "Disputes", glyph: "dispute" },
      { label: "Treasury", glyph: "treasury" },
    ],
  },
  login: {
    eyebrow: "Access board",
    title: "Role lane / secure cookie / redirect",
    glyph: "login",
    scenes: [
      { label: "Buyer", glyph: "buyer" },
      { label: "Provider", glyph: "provider" },
      { label: "Ops", glyph: "ops" },
    ],
  },
};

export function PortalPoster({
  theme,
  signal,
  actions = [],
}: {
  theme: PortalTheme;
  signal: string;
  actions?: Array<{ label: string }>;
}) {
  const config = posterConfig[theme];
  const scenes =
    actions.length > 0
      ? actions.slice(0, 3).map((action) => ({
          label: action.label,
          glyph: resolveGlyphFromText(action.label),
        }))
      : config.scenes;

  return (
    <div className={`portal-poster portal-poster--${theme}`}>
      <div className="portal-poster__header">
        <div className="portal-poster__mark">
          <Glyph name={config.glyph} className="portal-poster__mark-icon" />
        </div>
        <div className="portal-poster__meta">
          <span className="portal-poster__eyebrow">{config.eyebrow}</span>
          <strong className="portal-poster__title">{config.title}</strong>
        </div>
      </div>

      <div className="portal-poster__radar" aria-hidden="true">
        <span className="portal-poster__ring portal-poster__ring--outer" />
        <span className="portal-poster__ring portal-poster__ring--mid" />
        <span className="portal-poster__ring portal-poster__ring--inner" />
        <div className="portal-poster__core">
          <Glyph name={config.glyph} className="portal-poster__core-icon" />
        </div>
      </div>

      <div className="portal-poster__signal">{signal}</div>

      <div className="portal-poster__scene-list">
        {scenes.map((scene) => (
          <div key={scene.label} className="portal-poster__scene">
            <Glyph name={scene.glyph} className="portal-poster__scene-icon" />
            <span>{scene.label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
