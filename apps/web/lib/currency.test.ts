import { describe, expect, it } from "bun:test";
import { budgetUsageColor, formatBudgetUsage, formatCents, formatCentsCompact, parseCurrencyInputToCents } from "./currency";

describe("currency utilities", () => {
  it("formats cents as dollars", () => {
    expect(formatCents(1500)).toBe("$15.00");
    expect(formatCents(0)).toBe("$0.00");
    expect(formatCents(99)).toBe("$0.99");
  });

  it("formats compact amounts", () => {
    expect(formatCentsCompact(150000)).toBe("$1.5k");
    expect(formatCentsCompact(150000000)).toBe("$1.5M");
    expect(formatCentsCompact(500)).toBe("$5.00");
  });

  it("formats budget usage percentage", () => {
    expect(formatBudgetUsage(500, 1000)).toBe("50%");
    expect(formatBudgetUsage(1000, 1000)).toBe("100%");
    expect(formatBudgetUsage(0, 1000)).toBe("0%");
  });

  it("handles zero budget", () => {
    expect(formatBudgetUsage(0, 0)).toBe("—");
  });

  it("returns appropriate colors", () => {
    expect(budgetUsageColor(1000, 1000)).toBe("text-red-600");
    expect(budgetUsageColor(850, 1000)).toBe("text-orange-500");
    expect(budgetUsageColor(600, 1000)).toBe("text-yellow-500");
    expect(budgetUsageColor(300, 1000)).toBe("text-green-600");
    expect(budgetUsageColor(0, 0)).toBe("text-gray-400");
  });

  it("parses dollar-form inputs into cents", () => {
    expect(parseCurrencyInputToCents("68")).toBe(6800);
    expect(parseCurrencyInputToCents("68.4")).toBe(6840);
    expect(parseCurrencyInputToCents("68.40")).toBe(6840);
    expect(parseCurrencyInputToCents(" 2410.00 ")).toBe(241000);
  });

  it("rejects malformed currency input", () => {
    expect(parseCurrencyInputToCents("")).toBeNull();
    expect(parseCurrencyInputToCents("0")).toBeNull();
    expect(parseCurrencyInputToCents("-1")).toBeNull();
    expect(parseCurrencyInputToCents("12.345")).toBeNull();
    expect(parseCurrencyInputToCents("abc")).toBeNull();
  });
});
