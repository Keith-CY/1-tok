import { describe, expect, it } from "bun:test";
import { getNotificationMeta, formatNotification } from "./notifications";

describe("notification utilities", () => {
  it("returns meta for order.created", () => {
    const meta = getNotificationMeta("order.created");
    expect(meta.label).toBe("Order Created");
    expect(meta.emoji).toBe("📦");
  });

  it("returns meta for dispute.opened", () => {
    const meta = getNotificationMeta("dispute.opened");
    expect(meta.label).toBe("Dispute Opened");
    expect(meta.color).toBe("text-red-500");
  });

  it("formats notification for display", () => {
    expect(formatNotification("milestone.settled")).toBe("✅ Milestone Settled");
  });

  it("formats budget wall", () => {
    expect(formatNotification("budget_wall.hit")).toBe("🚧 Budget Wall Hit");
  });

  it("formats all event types", () => {
    const events = [
      "order.created", "milestone.settled", "dispute.opened", "dispute.resolved",
      "rfq.awarded", "order.completed", "order.rated", "budget_wall.hit",
    ] as const;
    for (const event of events) {
      const result = formatNotification(event);
      expect(result.length).toBeGreaterThan(3);
    }
  });
});
