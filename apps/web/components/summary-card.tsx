interface SummaryCardProps {
  kicker: string;
  value: string;
  hint: string;
}

export function SummaryCard({ kicker, value, hint }: SummaryCardProps) {
  return (
    <article className="stat-card">
      <div className="stat-card__kicker">{kicker}</div>
      <div className="stat-card__value">{value}</div>
      <p className="stat-card__hint">{hint}</p>
    </article>
  );
}

