package discord

import (
	"fmt"
	"strings"
)

// PushNotifier formats marketplace events for Discord channel delivery.
type PushNotifier struct {
	// Send is the function that delivers a message to Discord.
	// In production, this calls the Discord API. For testing, it can be a mock.
	Send func(channelID, message string) error
	// DefaultChannelID is the channel to push notifications to.
	DefaultChannelID string
}

// NewPushNotifier creates a push notifier.
func NewPushNotifier(channelID string, sender func(string, string) error) *PushNotifier {
	return &PushNotifier{
		Send:             sender,
		DefaultChannelID: channelID,
	}
}

// FormatEvent formats a marketplace event for Discord.
func FormatEvent(event string, payload map[string]any) string {
	var sb strings.Builder

	switch event {
	case "order.created":
		sb.WriteString(fmt.Sprintf("📦 **New Order** `%v`\nBuyer: %v | Provider: %v",
			payload["orderId"], payload["buyerOrgId"], payload["providerOrgId"]))
	case "milestone.settled":
		sb.WriteString(fmt.Sprintf("✅ **Milestone Settled** Order `%v` / `%v`",
			payload["orderId"], payload["milestoneId"]))
	case "dispute.opened":
		sb.WriteString(fmt.Sprintf("⚠️ **Dispute Opened** Order `%v` / `%v`",
			payload["orderId"], payload["milestoneId"]))
	case "dispute.resolved":
		sb.WriteString(fmt.Sprintf("🤝 **Dispute Resolved** `%v` — %v",
			payload["disputeId"], payload["resolution"]))
	case "order.completed":
		sb.WriteString(fmt.Sprintf("🎉 **Order Completed** `%v`", payload["orderId"]))
	case "order.rated":
		sb.WriteString(fmt.Sprintf("⭐ **Order Rated** `%v` — Score: %v",
			payload["orderId"], payload["score"]))
	case "budget_wall.hit":
		sb.WriteString(fmt.Sprintf("🚧 **Budget Wall** Order `%v` / `%v` — needs top-up",
			payload["orderId"], payload["milestoneId"]))
	case "rfq.awarded":
		sb.WriteString(fmt.Sprintf("🏆 **RFQ Awarded** `%v` → Order `%v`",
			payload["rfqId"], payload["orderId"]))
	default:
		sb.WriteString(fmt.Sprintf("📬 **%s**", event))
		for k, v := range payload {
			sb.WriteString(fmt.Sprintf("\n%s: %v", k, v))
		}
	}

	return sb.String()
}

// Notify sends a formatted event to the default Discord channel.
func (p *PushNotifier) Notify(event string, _ string, payload map[string]any) error {
	if p.Send == nil || p.DefaultChannelID == "" {
		return nil
	}
	message := FormatEvent(event, payload)
	return p.Send(p.DefaultChannelID, message)
}
