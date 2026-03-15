package discord

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBot_Ping(t *testing.T) {
	bot := NewBotWithoutVerification()

	body := `{"type":1}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	bot.HandleInteraction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Type != ResponsePong {
		t.Errorf("type = %d, want pong", resp.Type)
	}
}

func TestBot_Command(t *testing.T) {
	bot := NewBotWithoutVerification()
	bot.Register("hello", func(data InteractionData) InteractionResponse {
		return TextResponse("Hello, world!")
	})

	body := `{"type":2,"data":{"name":"hello"}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	bot.HandleInteraction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Content != "Hello, world!" {
		t.Errorf("content = %s", resp.Data.Content)
	}
}

func TestBot_UnknownCommand(t *testing.T) {
	bot := NewBotWithoutVerification()

	body := `{"type":2,"data":{"name":"nonexistent"}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	bot.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Data.Content, "Unknown command") {
		t.Errorf("content = %s", resp.Data.Content)
	}
}

func TestBot_UnknownInteractionType(t *testing.T) {
	bot := NewBotWithoutVerification()

	body := `{"type":99}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	bot.HandleInteraction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestBot_InvalidJSON(t *testing.T) {
	bot := NewBotWithoutVerification()

	req := httptest.NewRequest("POST", "/interactions", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	bot.HandleInteraction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestBot_CommandWithOptions(t *testing.T) {
	bot := NewBotWithoutVerification()
	bot.Register("search", func(data InteractionData) InteractionResponse {
		query := GetOptionString(data.Options, "q")
		return TextResponse("Searching for: " + query)
	})

	body := `{"type":2,"data":{"name":"search","options":[{"name":"q","value":"gpu agents"}]}}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()
	bot.HandleInteraction(w, req)

	var resp InteractionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Content != "Searching for: gpu agents" {
		t.Errorf("content = %s", resp.Data.Content)
	}
}

func TestGetOptionString_Missing(t *testing.T) {
	result := GetOptionString(nil, "missing")
	if result != "" {
		t.Errorf("expected empty, got %s", result)
	}
}

func TestGetOptionInt(t *testing.T) {
	options := []InteractionOption{
		{Name: "count", Value: float64(42)},
	}
	result := GetOptionInt(options, "count")
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestGetOptionInt_Missing(t *testing.T) {
	result := GetOptionInt(nil, "missing")
	if result != 0 {
		t.Errorf("expected 0, got %d", result)
	}
}

func TestGetOptionInt_Int64(t *testing.T) {
	options := []InteractionOption{
		{Name: "count", Value: int64(99)},
	}
	result := GetOptionInt(options, "count")
	if result != 99 {
		t.Errorf("expected 99, got %d", result)
	}
}

func TestFormatListings_Empty(t *testing.T) {
	embed := FormatListings(nil)
	if embed.Description != "No listings found." {
		t.Errorf("description = %s", embed.Description)
	}
}

func TestFormatListings_WithData(t *testing.T) {
	listings := []ListingView{
		{ID: "l1", Title: "GPU Agent", Category: "compute", PriceCents: 5000, Tags: []string{"gpu", "fast"}},
		{ID: "l2", Title: "NLP Bot", Category: "ai", PriceCents: 2000},
	}
	embed := FormatListings(listings)
	if len(embed.Fields) != 2 {
		t.Errorf("fields = %d", len(embed.Fields))
	}
	if embed.Fields[0].Name != "GPU Agent" {
		t.Errorf("field name = %s", embed.Fields[0].Name)
	}
}

func TestEmbedResponse(t *testing.T) {
	resp := EmbedResponse(Embed{Title: "Test"})
	if resp.Type != ResponseChannelMessage {
		t.Errorf("type = %d", resp.Type)
	}
	if len(resp.Data.Embeds) != 1 {
		t.Errorf("embeds = %d", len(resp.Data.Embeds))
	}
}

func TestNewBot_InvalidKey(t *testing.T) {
	_, err := NewBot("invalid")
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestNewBot_WrongKeySize(t *testing.T) {
	_, err := NewBot("aabbccdd")
	if err == nil {
		t.Error("expected error for wrong key size")
	}
}

func TestBuiltinCommands(t *testing.T) {
	cmds := BuiltinCommands()
	if len(cmds) != 7 {
		t.Errorf("expected 6 commands, got %d", len(cmds))
	}
}

func TestGetOptionString_NonString(t *testing.T) {
	options := []InteractionOption{
		{Name: "num", Value: float64(42)},
	}
	result := GetOptionString(options, "num")
	if result != "42" {
		t.Errorf("expected '42', got %s", result)
	}
}

func TestVerifySignature_BadHex(t *testing.T) {
	result := verifySignature(nil, "not-hex", "ts", []byte("body"))
	if result {
		t.Error("expected false for bad hex")
	}
}

func TestBot_WithValidKey(t *testing.T) {
	// Generate a valid 32-byte hex public key
	key := strings.Repeat("ab", 32)
	bot, err := NewBot(key)
	if err != nil {
		t.Fatal(err)
	}
	if bot == nil {
		t.Error("expected non-nil bot")
	}
}

func TestBot_InvalidSignature(t *testing.T) {
	key := strings.Repeat("ab", 32)
	bot, _ := NewBot(key)

	body := `{"type":1}`
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(body))
	req.Header.Set("X-Signature-Ed25519", "deadbeef")
	req.Header.Set("X-Signature-Timestamp", "12345")
	w := httptest.NewRecorder()
	bot.HandleInteraction(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

type errorReader struct{}
func (errorReader) Read(p []byte) (int, error) { return 0, fmt.Errorf("read error") }

func TestBot_EmptyBody(t *testing.T) {
	bot := NewBotWithoutVerification()
	req := httptest.NewRequest("POST", "/interactions", strings.NewReader(""))
	w := httptest.NewRecorder()
	bot.HandleInteraction(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
