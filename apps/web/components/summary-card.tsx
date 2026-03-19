import { Glyph, resolveGlyphFromText } from "./portal-visuals";
import { InfoIcon } from "./ui";

interface SummaryCardProps {
  kicker: string;
  value: string;
  hint: string;
}

export function SummaryCard({ kicker, value, hint }: SummaryCardProps) {
  const glyph = resolveGlyphFromText(kicker);

  return (
    <article className="stat-card stat-card--visual">
      <div className="stat-card__top">
        <div className="stat-card__glyph">
          <Glyph name={glyph} className="stat-card__glyph-icon" />
        </div>
        <div className="stat-card__meta">
          <div className="stat-card__kicker">{kicker}</div>
          <div className="stat-card__value">{value}</div>
        </div>
        <InfoIcon label={hint} />
      </div>
      <div className="stat-card__track" aria-hidden="true">
        <span />
        <span />
        <span />
      </div>
      <p className="stat-card__hint">{hint}</p>
    </article>
  );
}
