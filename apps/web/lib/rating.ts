// Rating display utilities for the web portal.

/**
 * Formats a numeric rating as stars.
 * @example formatStars(4.5) → "★★★★☆"
 */
export function formatStars(rating: number, max = 5): string {
  const rounded = Math.round(rating);
  return "★".repeat(rounded) + "☆".repeat(max - rounded);
}

/**
 * Formats a rating with count.
 * @example formatRating(4.5, 23) → "★★★★☆ (4.5 · 23 reviews)"
 */
export function formatRating(rating: number, count: number): string {
  if (count === 0) return "No ratings yet";
  return `${formatStars(rating)} (${rating.toFixed(1)} · ${count} review${count === 1 ? "" : "s"})`;
}

/**
 * Returns a CSS color class for a rating tier.
 */
export function ratingColor(rating: number): string {
  if (rating >= 4.5) return "text-green-600";
  if (rating >= 3.5) return "text-yellow-600";
  if (rating >= 2.5) return "text-orange-500";
  return "text-red-500";
}
