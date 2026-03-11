package fiber

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("fiber client is not configured")

type InvoiceClient interface {
	CreateInvoice(ctx context.Context, input CreateInvoiceInput) (CreateInvoiceResult, error)
	GetInvoiceStatus(ctx context.Context, invoice string) (InvoiceStatusResult, error)
	QuotePayout(ctx context.Context, input QuotePayoutInput) (QuotePayoutResult, error)
	RequestPayout(ctx context.Context, input RequestPayoutInput) (RequestPayoutResult, error)
}

type CreateInvoiceInput struct {
	PostID     string `json:"postId"`
	FromUserID string `json:"fromUserId"`
	ToUserID   string `json:"toUserId"`
	Asset      string `json:"asset"`
	Amount     string `json:"amount"`
	Message    string `json:"message,omitempty"`
}

type CreateInvoiceResult struct {
	Invoice string `json:"invoice"`
}

type InvoiceStatusResult struct {
	State string `json:"state"`
}

type WithdrawalDestination struct {
	Kind           string `json:"kind"`
	Address        string `json:"address,omitempty"`
	PaymentRequest string `json:"paymentRequest,omitempty"`
}

type QuotePayoutInput struct {
	UserID      string                `json:"userId"`
	Asset       string                `json:"asset"`
	Amount      string                `json:"amount"`
	Destination WithdrawalDestination `json:"destination"`
}

type QuotePayoutResult struct {
	Asset             string  `json:"asset"`
	Amount            string  `json:"amount"`
	MinimumAmount     string  `json:"minimumAmount"`
	AvailableBalance  string  `json:"availableBalance"`
	LockedBalance     string  `json:"lockedBalance"`
	NetworkFee        string  `json:"networkFee"`
	ReceiveAmount     string  `json:"receiveAmount"`
	DestinationValid  bool    `json:"destinationValid"`
	ValidationMessage *string `json:"validationMessage"`
}

type RequestPayoutInput struct {
	UserID      string                `json:"userId"`
	Asset       string                `json:"asset"`
	Amount      string                `json:"amount"`
	Destination WithdrawalDestination `json:"destination"`
}

type RequestPayoutResult struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type Client struct {
	endpoint   string
	appID      string
	secret     string
	httpClient *http.Client
}

type missingClient struct {
	err error
}

func NewClient(endpoint, appID, secret string) *Client {
	return &Client{
		endpoint:   endpoint,
		appID:      appID,
		secret:     secret,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func NewClientFromEnv() InvoiceClient {
	endpoint := strings.TrimSpace(os.Getenv("FIBER_RPC_URL"))
	appID := strings.TrimSpace(os.Getenv("FIBER_APP_ID"))
	secret := strings.TrimSpace(os.Getenv("FIBER_HMAC_SECRET"))
	if endpoint == "" || appID == "" || secret == "" {
		return missingClient{err: ErrNotConfigured}
	}
	return NewClient(endpoint, appID, secret)
}

func (c *Client) CreateInvoice(ctx context.Context, input CreateInvoiceInput) (CreateInvoiceResult, error) {
	var result CreateInvoiceResult
	if err := c.call(ctx, "tip.create", input, &result); err != nil {
		return CreateInvoiceResult{}, err
	}
	return result, nil
}

func (c *Client) GetInvoiceStatus(ctx context.Context, invoice string) (InvoiceStatusResult, error) {
	var result InvoiceStatusResult
	if err := c.call(ctx, "tip.status", map[string]string{"invoice": invoice}, &result); err != nil {
		return InvoiceStatusResult{}, err
	}
	return result, nil
}

func (c *Client) QuotePayout(ctx context.Context, input QuotePayoutInput) (QuotePayoutResult, error) {
	var result QuotePayoutResult
	if err := c.call(ctx, "withdrawal.quote", input, &result); err != nil {
		return QuotePayoutResult{}, err
	}
	return result, nil
}

func (c *Client) RequestPayout(ctx context.Context, input RequestPayoutInput) (RequestPayoutResult, error) {
	var result RequestPayoutResult
	if err := c.call(ctx, "withdrawal.request", input, &result); err != nil {
		return RequestPayoutResult{}, err
	}
	return result, nil
}

func (c *Client) call(ctx context.Context, method string, params any, target any) error {
	if strings.TrimSpace(c.endpoint) == "" || strings.TrimSpace(c.appID) == "" || strings.TrimSpace(c.secret) == "" {
		return ErrNotConfigured
	}

	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      newRPCID(),
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}

	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := newNonce()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-app-id", c.appID)
	req.Header.Set("x-ts", ts)
	req.Header.Set("x-nonce", nonce)
	req.Header.Set("x-signature", signPayload(c.secret, payload, ts, nonce))

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("fiber rpc returned %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var rpc struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &rpc); err != nil {
		return err
	}
	if rpc.Error != nil {
		return fmt.Errorf("fiber rpc error %d: %s", rpc.Error.Code, rpc.Error.Message)
	}
	if len(rpc.Result) == 0 {
		return errors.New("fiber rpc response missing result")
	}
	return json.Unmarshal(rpc.Result, target)
}

func (m missingClient) CreateInvoice(context.Context, CreateInvoiceInput) (CreateInvoiceResult, error) {
	return CreateInvoiceResult{}, m.err
}

func (m missingClient) GetInvoiceStatus(context.Context, string) (InvoiceStatusResult, error) {
	return InvoiceStatusResult{}, m.err
}

func (m missingClient) QuotePayout(context.Context, QuotePayoutInput) (QuotePayoutResult, error) {
	return QuotePayoutResult{}, m.err
}

func (m missingClient) RequestPayout(context.Context, RequestPayoutInput) (RequestPayoutResult, error) {
	return RequestPayoutResult{}, m.err
}

func signPayload(secret string, payload []byte, ts, nonce string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ts))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(nonce))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func newNonce() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("nonce-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}

func newRPCID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
