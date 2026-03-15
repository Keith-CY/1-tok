// Notification display utilities for the web portal.
import type { NotificationEvent } from "@1tok/contracts";

interface NotificationMeta {
  label: string;
  emoji: string;
  color: string;
}

const eventMeta: Record<NotificationEvent, NotificationMeta> = {
  "order.created": { label: "Order Created", emoji: "📦", color: "text-blue-600" },
  "milestone.settled": { label: "Milestone Settled", emoji: "✅", color: "text-green-600" },
  "dispute.opened": { label: "Dispute Opened", emoji: "⚠️", color: "text-red-500" },
  "dispute.resolved": { label: "Dispute Resolved", emoji: "🤝", color: "text-green-600" },
  "rfq.awarded": { label: "RFQ Awarded", emoji: "🏆", color: "text-yellow-500" },
  "order.completed": { label: "Order Completed", emoji: "🎉", color: "text-green-600" },
  "order.rated": { label: "Order Rated", emoji: "⭐", color: "text-yellow-500" },
  "budget_wall.hit": { label: "Budget Wall Hit", emoji: "🚧", color: "text-orange-500" },
};

/**
 * Returns display metadata for a notification event type.
 */
export function getNotificationMeta(event: NotificationEvent): NotificationMeta {
  return eventMeta[event] ?? { label: event, emoji: "📬", color: "text-gray-500" };
}

/**
 * Formats a notification for display.
 */
export function formatNotification(event: NotificationEvent): string {
  const meta = getNotificationMeta(event);
  return `${meta.emoji} ${meta.label}`;
}
