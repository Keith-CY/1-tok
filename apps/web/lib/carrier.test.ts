import { describe, expect, it } from "bun:test";
import { formatJobState, formatProgress, getJobStateMeta } from "./carrier";

describe("carrier utilities", () => {
  it("formats job states", () => {
    expect(formatJobState("pending")).toBe("⏳ Pending");
    expect(formatJobState("running")).toBe("🔄 Running");
    expect(formatJobState("completed")).toBe("✅ Completed");
    expect(formatJobState("failed")).toBe("❌ Failed");
    expect(formatJobState("cancelled")).toBe("🚫 Cancelled");
  });

  it("returns colors for states", () => {
    expect(getJobStateMeta("completed").color).toBe("text-green-600");
    expect(getJobStateMeta("failed").color).toBe("text-red-500");
  });

  it("formats progress bar", () => {
    expect(formatProgress(5, 10)).toBe("█████░░░░░ 50%");
    expect(formatProgress(10, 10)).toBe("██████████ 100%");
    expect(formatProgress(0, 10)).toBe("░░░░░░░░░░ 0%");
  });

  it("handles zero total", () => {
    expect(formatProgress(0, 0)).toBe("0%");
  });
});
