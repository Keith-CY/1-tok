// Package discord provides a Discord Bot command handler for the 1-tok marketplace.
package discord

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var (
	ErrInvalidSignature = errors.New("invalid request signature")
	ErrUnknownCommand   = errors.New("unknown command")
)

// InteractionType represents the Discord interaction type.
type InteractionType int

const (
	InteractionPing               InteractionType = 1
	InteractionApplicationCommand InteractionType = 2
)

// ResponseType represents the Discord interaction response type.
type ResponseType int

const (
	ResponsePong                 ResponseType = 1
	ResponseChannelMessage       ResponseType = 4
)

// Interaction represents a Discord interaction payload.
type Interaction struct {
	Type InteractionType        `json:"type"`
	Data InteractionData        `json:"data,omitempty"`
}

// InteractionData holds command data.
type InteractionData struct {
	Name    string              `json:"name"`
	Options []InteractionOption `json:"options,omitempty"`
}

// InteractionOption holds a command option.
type InteractionOption struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// InteractionResponse is the response to a Discord interaction.
type InteractionResponse struct {
	Type ResponseType           `json:"type"`
	Data *InteractionResponseData `json:"data,omitempty"`
}

// InteractionResponseData holds the response content.
type InteractionResponseData struct {
	Content string  `json:"content,omitempty"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

// Embed represents a Discord embed.
type Embed struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color,omitempty"`
	Fields      []EmbedField `json:"fields,omitempty"`
}

// EmbedField represents a field in a Discord embed.
type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// CommandHandler handles a specific slash command.
type CommandHandler func(data InteractionData) InteractionResponse

// Bot is the Discord bot command handler.
type Bot struct {
	publicKey ed25519.PublicKey
	commands  map[string]CommandHandler
}

// NewBot creates a new Discord bot with the given public key (hex-encoded).
func NewBot(publicKeyHex string) (*Bot, error) {
	key, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	if len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key must be %d bytes", ed25519.PublicKeySize)
	}
	return &Bot{
		publicKey: ed25519.PublicKey(key),
		commands:  make(map[string]CommandHandler),
	}, nil
}

// NewBotWithoutVerification creates a bot that skips signature verification (for testing).
func NewBotWithoutVerification() *Bot {
	return &Bot{
		commands: make(map[string]CommandHandler),
	}
}

// Register adds a command handler.
func (b *Bot) Register(name string, handler CommandHandler) {
	b.commands[name] = handler
}

// HandleInteraction processes an HTTP interaction request.
func (b *Bot) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB max
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Verify signature if public key is set
	if b.publicKey != nil {
		signature := r.Header.Get("X-Signature-Ed25519")
		timestamp := r.Header.Get("X-Signature-Timestamp")
		if !verifySignature(b.publicKey, signature, timestamp, body) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var interaction Interaction
	if err := json.Unmarshal(body, &interaction); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var response InteractionResponse

	switch interaction.Type {
	case InteractionPing:
		response = InteractionResponse{Type: ResponsePong}
	case InteractionApplicationCommand:
		response = b.handleCommand(interaction.Data)
	default:
		http.Error(w, "unknown interaction type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (b *Bot) handleCommand(data InteractionData) InteractionResponse {
	handler, ok := b.commands[data.Name]
	if !ok {
		return TextResponse(fmt.Sprintf("Unknown command: %s", data.Name))
	}
	return handler(data)
}

// TextResponse creates a simple text response.
func TextResponse(content string) InteractionResponse {
	return InteractionResponse{
		Type: ResponseChannelMessage,
		Data: &InteractionResponseData{Content: content},
	}
}

// EmbedResponse creates an embed response.
func EmbedResponse(embeds ...Embed) InteractionResponse {
	return InteractionResponse{
		Type: ResponseChannelMessage,
		Data: &InteractionResponseData{Embeds: embeds},
	}
}

func verifySignature(key ed25519.PublicKey, signatureHex, timestamp string, body []byte) bool {
	sig, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	msg := append([]byte(timestamp), body...)
	return ed25519.Verify(key, msg, sig)
}

// GetOptionString extracts a string option by name.
func GetOptionString(options []InteractionOption, name string) string {
	for _, opt := range options {
		if opt.Name == name {
			if s, ok := opt.Value.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", opt.Value)
		}
	}
	return ""
}

// GetOptionInt extracts an integer option by name.
func GetOptionInt(options []InteractionOption, name string) int64 {
	for _, opt := range options {
		if opt.Name == name {
			switch v := opt.Value.(type) {
			case float64:
				return int64(v)
			case int64:
				return v
			}
		}
	}
	return 0
}

// SlashCommandDef defines a slash command for registration with Discord.
type SlashCommandDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        int    `json:"type,omitempty"` // 1 = CHAT_INPUT
}

// BuiltinCommands returns the slash command definitions for the marketplace.
func BuiltinCommands() []SlashCommandDef {
	return []SlashCommandDef{
		{Name: "listings", Description: "Browse available agent listings", Type: 1},
		{Name: "rfq-create", Description: "Create a new Request for Quote", Type: 1},
		{Name: "rfq-status", Description: "Check status of an RFQ", Type: 1},
		{Name: "bids", Description: "View bids on an RFQ", Type: 1},
		{Name: "award", Description: "Award an RFQ to a bid", Type: 1},
		{Name: "order-status", Description: "Check order status", Type: 1},
		{Name: "stats", Description: "View marketplace statistics", Type: 1},
	}
}

// FormatListings formats listings for Discord display.
func FormatListings(listings []ListingView) Embed {
	if len(listings) == 0 {
		return Embed{
			Title:       "📋 Listings",
			Description: "No listings found.",
			Color:       0x5865F2,
		}
	}

	fields := make([]EmbedField, 0, len(listings))
	for _, l := range listings {
		tags := ""
		if len(l.Tags) > 0 {
			tags = " | " + strings.Join(l.Tags, ", ")
		}
		fields = append(fields, EmbedField{
			Name:   l.Title,
			Value:  fmt.Sprintf("💰 %d cents | 📂 %s%s", l.PriceCents, l.Category, tags),
			Inline: false,
		})
	}

	return Embed{
		Title:  "📋 Listings",
		Color:  0x5865F2,
		Fields: fields,
	}
}

// ListingView is a simplified listing for Discord display.
type ListingView struct {
	ID         string
	Title      string
	Category   string
	PriceCents int64
	Tags       []string
}
