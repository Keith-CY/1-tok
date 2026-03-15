package discord

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/platform"
)

func newTestBot() *MarketplaceBot {
	app := platform.NewAppWithMemory()
	return NewMarketplaceBot(app)
}

func TestMarketplaceBot_Listings(t *testing.T) {
	mb := newTestBot()

	body := `{"type":2,"data":{"name":"listings"}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data == nil || len(resp.Data.Embeds) == 0 {
		t.Error("expected embed response")
	}
}

func TestMarketplaceBot_ListingsWithQuery(t *testing.T) {
	mb := newTestBot()

	body := `{"type":2,"data":{"name":"listings","options":[{"name":"q","value":"nonexistent"}]}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Embeds[0].Description != "No listings found." {
		t.Errorf("expected no listings, got %s", resp.Data.Embeds[0].Description)
	}
}

func TestMarketplaceBot_ListingsWithTag(t *testing.T) {
	mb := newTestBot()

	body := `{"type":2,"data":{"name":"listings","options":[{"name":"tag","value":"some-tag"}]}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	// Should not error
	if resp.Type != ResponseChannelMessage {
		t.Errorf("type = %d", resp.Type)
	}
}

func TestMarketplaceBot_OrderStatus(t *testing.T) {
	mb := newTestBot()

	// Create an order first
	rfq, _ := mb.app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Bot test", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := mb.app.CreateBid(rfq.ID, platform.CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid",
		QuoteCents: 5000, Milestones: []platform.BidMilestoneInput{
			{ID: "ms_1", Title: "Work", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	_, order, _ := mb.app.AwardRFQ(rfq.ID, platform.AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	body := `{"type":2,"data":{"name":"order-status","options":[{"name":"order_id","value":"` + order.ID + `"}]}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Embeds) == 0 {
		t.Fatal("expected embed")
	}
	if !strings.Contains(resp.Data.Embeds[0].Title, order.ID) {
		t.Errorf("title = %s", resp.Data.Embeds[0].Title)
	}
}

func TestMarketplaceBot_OrderStatus_NotFound(t *testing.T) {
	mb := newTestBot()

	body := `{"type":2,"data":{"name":"order-status","options":[{"name":"order_id","value":"nonexistent"}]}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Data.Content, "not found") {
		t.Errorf("content = %s", resp.Data.Content)
	}
}

func TestMarketplaceBot_OrderStatus_MissingID(t *testing.T) {
	mb := newTestBot()

	body := `{"type":2,"data":{"name":"order-status"}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Data.Content, "order_id") {
		t.Errorf("content = %s", resp.Data.Content)
	}
}

func TestMarketplaceBot_RFQStatus(t *testing.T) {
	mb := newTestBot()

	rfq, _ := mb.app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID: "org_b", Title: "RFQ Bot", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	body := `{"type":2,"data":{"name":"rfq-status","options":[{"name":"rfq_id","value":"` + rfq.ID + `"}]}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Embeds) == 0 {
		t.Fatal("expected embed")
	}
}

func TestMarketplaceBot_RFQStatus_NotFound(t *testing.T) {
	mb := newTestBot()

	body := `{"type":2,"data":{"name":"rfq-status","options":[{"name":"rfq_id","value":"nonexistent"}]}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Data.Content, "not found") {
		t.Errorf("content = %s", resp.Data.Content)
	}
}

func TestMarketplaceBot_RFQStatus_MissingID(t *testing.T) {
	mb := newTestBot()

	body := `{"type":2,"data":{"name":"rfq-status"}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Data.Content, "rfq_id") {
		t.Errorf("content = %s", resp.Data.Content)
	}
}

func TestMarketplaceBot_Bids(t *testing.T) {
	mb := newTestBot()

	rfq, _ := mb.app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Bids Bot", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	mb.app.CreateBid(rfq.ID, platform.CreateBidInput{
		ProviderOrgID: "org_p", Message: "my bid",
		QuoteCents: 4000, Milestones: []platform.BidMilestoneInput{
			{ID: "ms_1", Title: "Work", BasePriceCents: 4000, BudgetCents: 4000},
		},
	})

	body := `{"type":2,"data":{"name":"bids","options":[{"name":"rfq_id","value":"` + rfq.ID + `"}]}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Embeds) == 0 {
		t.Fatal("expected embed")
	}
	if len(resp.Data.Embeds[0].Fields) == 0 {
		t.Error("expected bid fields")
	}
}

func TestMarketplaceBot_Bids_Empty(t *testing.T) {
	mb := newTestBot()

	rfq, _ := mb.app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID: "org_b", Title: "No bids", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	body := `{"type":2,"data":{"name":"bids","options":[{"name":"rfq_id","value":"` + rfq.ID + `"}]}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Data.Content, "No bids") {
		t.Errorf("content = %s", resp.Data.Content)
	}
}

func TestMarketplaceBot_Bids_MissingID(t *testing.T) {
	mb := newTestBot()

	body := `{"type":2,"data":{"name":"bids"}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Data.Content, "rfq_id") {
		t.Errorf("content = %s", resp.Data.Content)
	}
}

func TestMarketplaceBot_Stats(t *testing.T) {
	mb := newTestBot()

	body := `{"type":2,"data":{"name":"stats"}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Embeds) == 0 {
		t.Fatal("expected embed")
	}
	if resp.Data.Embeds[0].Title != "📊 Marketplace Stats" {
		t.Errorf("title = %s", resp.Data.Embeds[0].Title)
	}
}

func TestMarketplaceBot_Leaderboard(t *testing.T) {
	mb := newTestBot()

	body := `{"type":2,"data":{"name":"leaderboard"}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	mb.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Embeds) == 0 {
		t.Fatal("expected embed")
	}
	if resp.Data.Embeds[0].Title != "🏆 Provider Leaderboard" {
		t.Errorf("title = %s", resp.Data.Embeds[0].Title)
	}
}
