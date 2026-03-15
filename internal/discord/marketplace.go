package discord

import (
	"fmt"
	"strings"

	"github.com/chenyu/1-tok/internal/platform"
)

// MarketplaceBot wraps Bot with platform.App to handle real marketplace commands.
type MarketplaceBot struct {
	*Bot
	app *platform.App
}

// NewMarketplaceBot creates a Discord bot wired to the marketplace platform.
func NewMarketplaceBot(app *platform.App) *MarketplaceBot {
	mb := &MarketplaceBot{
		Bot: NewBotWithoutVerification(),
		app: app,
	}
	mb.registerCommands()
	return mb
}

func (mb *MarketplaceBot) registerCommands() {
	mb.Register("listings", mb.handleListings)
	mb.Register("order-status", mb.handleOrderStatus)
	mb.Register("rfq-status", mb.handleRFQStatus)
	mb.Register("bids", mb.handleBids)
	mb.Register("stats", mb.handleStats)
}

func (mb *MarketplaceBot) handleListings(data InteractionData) InteractionResponse {
	input := platform.ListListingsInput{
		Query:    GetOptionString(data.Options, "q"),
		Category: GetOptionString(data.Options, "category"),
	}
	if tag := GetOptionString(data.Options, "tag"); tag != "" {
		input.Tags = []string{tag}
	}

	listings, err := mb.app.SearchListings(input)
	if err != nil {
		return TextResponse(fmt.Sprintf("❌ Error: %s", err))
	}

	views := make([]ListingView, 0, len(listings))
	for _, l := range listings {
		views = append(views, ListingView{
			ID:         l.ID,
			Title:      l.Title,
			Category:   l.Category,
			PriceCents: l.BasePriceCents,
			Tags:       l.Tags,
		})
	}

	return EmbedResponse(FormatListings(views))
}

func (mb *MarketplaceBot) handleOrderStatus(data InteractionData) InteractionResponse {
	orderID := GetOptionString(data.Options, "order_id")
	if orderID == "" {
		return TextResponse("❌ Please provide an order_id")
	}

	order, err := mb.app.GetOrder(orderID)
	if err != nil {
		return TextResponse(fmt.Sprintf("❌ Order not found: %s", orderID))
	}

	fields := []EmbedField{
		{Name: "Status", Value: string(order.Status), Inline: true},
		{Name: "Buyer", Value: order.BuyerOrgID, Inline: true},
		{Name: "Provider", Value: order.ProviderOrgID, Inline: true},
		{Name: "Funding", Value: string(order.FundingMode), Inline: true},
	}

	for _, ms := range order.Milestones {
		fields = append(fields, EmbedField{
			Name:  fmt.Sprintf("📌 %s", ms.Title),
			Value: fmt.Sprintf("State: %s | Budget: %d¢ | Settled: %d¢", ms.State, ms.BudgetCents, ms.SettledCents),
		})
	}

	color := 0x5865F2 // blue
	switch order.Status {
	case "completed":
		color = 0x57F287 // green
	case "awaiting_budget":
		color = 0xFEE75C // yellow
	case "disputed":
		color = 0xED4245 // red
	}

	return EmbedResponse(Embed{
		Title:  fmt.Sprintf("📦 Order %s", order.ID),
		Color:  color,
		Fields: fields,
	})
}

func (mb *MarketplaceBot) handleRFQStatus(data InteractionData) InteractionResponse {
	rfqID := GetOptionString(data.Options, "rfq_id")
	if rfqID == "" {
		return TextResponse("❌ Please provide an rfq_id")
	}

	rfq, err := mb.app.GetRFQ(rfqID)
	if err != nil {
		return TextResponse(fmt.Sprintf("❌ RFQ not found: %s", rfqID))
	}

	fields := []EmbedField{
		{Name: "Title", Value: rfq.Title, Inline: true},
		{Name: "Status", Value: string(rfq.Status), Inline: true},
		{Name: "Category", Value: rfq.Category, Inline: true},
		{Name: "Budget", Value: fmt.Sprintf("%d¢", rfq.BudgetCents), Inline: true},
		{Name: "Buyer", Value: rfq.BuyerOrgID, Inline: true},
	}
	if rfq.AwardedBidID != "" {
		fields = append(fields, EmbedField{
			Name: "Awarded To", Value: rfq.AwardedProviderOrgID, Inline: true,
		})
	}

	return EmbedResponse(Embed{
		Title:  fmt.Sprintf("📋 RFQ %s", rfq.ID),
		Color:  0x5865F2,
		Fields: fields,
	})
}

func (mb *MarketplaceBot) handleBids(data InteractionData) InteractionResponse {
	rfqID := GetOptionString(data.Options, "rfq_id")
	if rfqID == "" {
		return TextResponse("❌ Please provide an rfq_id")
	}

	bids, err := mb.app.ListBids(rfqID)
	if err != nil {
		return TextResponse(fmt.Sprintf("❌ Error: %s", err))
	}

	if len(bids) == 0 {
		return TextResponse(fmt.Sprintf("No bids yet on RFQ %s", rfqID))
	}

	fields := make([]EmbedField, 0, len(bids))
	for _, bid := range bids {
		milestones := make([]string, 0, len(bid.Milestones))
		for _, ms := range bid.Milestones {
			milestones = append(milestones, ms.Title)
		}
		fields = append(fields, EmbedField{
			Name:  fmt.Sprintf("%s — %s", bid.ProviderOrgID, string(bid.Status)),
			Value: fmt.Sprintf("💰 %d¢ | Milestones: %s", bid.QuoteCents, strings.Join(milestones, ", ")),
		})
	}

	return EmbedResponse(Embed{
		Title:  fmt.Sprintf("📊 Bids on RFQ %s", rfqID),
		Color:  0x5865F2,
		Fields: fields,
	})
}

func (mb *MarketplaceBot) handleStats(data InteractionData) InteractionResponse {
	stats, err := mb.app.GetMarketplaceStats()
	if err != nil {
		return TextResponse(fmt.Sprintf("❌ Error: %s", err))
	}

	avgRating := "—"
	if stats.TotalRatings > 0 {
		avgRating = fmt.Sprintf("%.1f ⭐", stats.AverageRating)
	}

	return EmbedResponse(Embed{
		Title: "📊 Marketplace Stats",
		Color: 0x5865F2,
		Fields: []EmbedField{
			{Name: "Providers", Value: fmt.Sprintf("%d", stats.TotalProviders), Inline: true},
			{Name: "Listings", Value: fmt.Sprintf("%d", stats.TotalListings), Inline: true},
			{Name: "RFQs", Value: fmt.Sprintf("%d total / %d open", stats.TotalRFQs, stats.OpenRFQs), Inline: true},
			{Name: "Orders", Value: fmt.Sprintf("%d total / %d active", stats.TotalOrders, stats.ActiveOrders), Inline: true},
			{Name: "Disputes", Value: fmt.Sprintf("%d total / %d open", stats.TotalDisputes, stats.OpenDisputes), Inline: true},
			{Name: "Avg Rating", Value: avgRating, Inline: true},
		},
	})
}
