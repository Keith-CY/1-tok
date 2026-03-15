import { describe, expect, it } from "bun:test";
import { formatStars, formatRating, ratingColor } from "./rating";

describe("rating utilities", () => {
  it("formats stars for full rating", () => {
    expect(formatStars(5)).toBe("★★★★★");
  });

  it("formats stars for partial rating", () => {
    expect(formatStars(3)).toBe("★★★☆☆");
  });

  it("rounds to nearest star", () => {
    expect(formatStars(4.5)).toBe("★★★★★");
    expect(formatStars(4.4)).toBe("★★★★☆");
  });

  it("formats rating with count", () => {
    expect(formatRating(4.5, 23)).toBe("★★★★★ (4.5 · 23 reviews)");
  });

  it("handles single review", () => {
    expect(formatRating(5, 1)).toBe("★★★★★ (5.0 · 1 review)");
  });

  it("handles no ratings", () => {
    expect(formatRating(0, 0)).toBe("No ratings yet");
  });

  it("returns green for high ratings", () => {
    expect(ratingColor(4.8)).toBe("text-green-600");
  });

  it("returns yellow for medium ratings", () => {
    expect(ratingColor(3.8)).toBe("text-yellow-600");
  });

  it("returns orange for low ratings", () => {
    expect(ratingColor(2.8)).toBe("text-orange-500");
  });

  it("returns red for very low ratings", () => {
    expect(ratingColor(1.5)).toBe("text-red-500");
  });
});
