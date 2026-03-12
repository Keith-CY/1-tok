package release

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type AbuseConfig struct {
	IAMBaseURL    string
	SentryBaseURL string
	ForwardedIP   string
}

type AbuseSummary struct {
	Attempts         int  `json:"attempts"`
	RateLimited      bool `json:"rateLimited"`
	SentryEventCount int  `json:"sentryEventCount"`
}

func AbuseConfigFromEnv() AbuseConfig {
	return AbuseConfig{
		IAMBaseURL:    envOrDefault("RELEASE_ABUSE_IAM_BASE_URL", "http://127.0.0.1:8081"),
		SentryBaseURL: envOrDefault("RELEASE_ABUSE_SENTRY_BASE_URL", "http://127.0.0.1:8092"),
		ForwardedIP:   envOrDefault("RELEASE_ABUSE_FORWARDED_IP", "203.0.113.10"),
	}
}

func RunAbuseSmoke(ctx context.Context, cfg AbuseConfig) (AbuseSummary, error) {
	if strings.TrimSpace(cfg.IAMBaseURL) == "" || strings.TrimSpace(cfg.SentryBaseURL) == "" {
		return AbuseSummary{}, errors.New("iam and sentry base urls are required")
	}
	if strings.TrimSpace(cfg.ForwardedIP) == "" {
		cfg.ForwardedIP = "203.0.113.10"
	}
	client := &http.Client{Timeout: 10 * time.Second}

	if err := healthcheck(ctx, client, cfg.IAMBaseURL); err != nil {
		return AbuseSummary{}, fmt.Errorf("iam health: %w", err)
	}
	if err := healthcheck(ctx, client, cfg.SentryBaseURL); err != nil {
		return AbuseSummary{}, fmt.Errorf("sentry health: %w", err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	email := "abuse-" + suffix + "@example.com"
	password := "correct horse battery staple"
	if err := signupAbuseUser(ctx, client, cfg.IAMBaseURL, cfg.ForwardedIP, email, password); err != nil {
		return AbuseSummary{}, err
	}

	summary := AbuseSummary{}
	for attempt := 1; attempt <= 11; attempt++ {
		status, err := loginAttempt(ctx, client, cfg.IAMBaseURL, cfg.ForwardedIP, email, password)
		if err != nil {
			return AbuseSummary{}, err
		}
		summary.Attempts = attempt
		if status == http.StatusTooManyRequests {
			summary.RateLimited = true
			break
		}
		if status != http.StatusCreated {
			return AbuseSummary{}, fmt.Errorf("unexpected login status %d", status)
		}
	}
	if !summary.RateLimited {
		return AbuseSummary{}, errors.New("abuse smoke: login flood did not return 429")
	}

	count, err := waitForSentryEvents(ctx, client, cfg.SentryBaseURL)
	if err != nil {
		return AbuseSummary{}, err
	}
	summary.SentryEventCount = count

	return summary, nil
}

func signupAbuseUser(ctx context.Context, client *http.Client, iamBaseURL, forwardedIP, email, password string) error {
	payload := map[string]any{
		"email":            email,
		"password":         password,
		"name":             "Abuse Smoke",
		"organizationName": "Abuse Buyer",
		"organizationKind": "buyer",
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(iamBaseURL, "/")+"/v1/signup", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(forwardedIP) != "" {
		req.Header.Set("X-Forwarded-For", forwardedIP)
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("signup status %d", res.StatusCode)
	}
	return nil
}

func loginAttempt(ctx context.Context, client *http.Client, iamBaseURL, forwardedIP, email, password string) (int, error) {
	payload := map[string]any{
		"email":    email,
		"password": password,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(iamBaseURL, "/")+"/v1/sessions", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(forwardedIP) != "" {
		req.Header.Set("X-Forwarded-For", forwardedIP)
	}

	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	return res.StatusCode, nil
}

func waitForSentryEvents(ctx context.Context, client *http.Client, sentryBaseURL string) (int, error) {
	for attempt := 0; attempt < 10; attempt++ {
		count, err := sentryEventCount(ctx, client, sentryBaseURL)
		if err != nil {
			return 0, err
		}
		if count > 0 {
			return count, nil
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return 0, errors.New("abuse smoke: expected sentry event after rate limit")
}

func sentryEventCount(ctx context.Context, client *http.Client, sentryBaseURL string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(sentryBaseURL, "/")+"/events", nil)
	if err != nil {
		return 0, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("sentry events status %d", res.StatusCode)
	}
	var payload struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return 0, err
	}
	return payload.Count, nil
}

func healthcheck(ctx context.Context, client *http.Client, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/healthz", nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", res.StatusCode)
	}
	return nil
}
