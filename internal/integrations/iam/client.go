package iam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	ErrNotConfigured = errors.New("iam client is not configured")
	ErrUnauthorized  = errors.New("iam unauthorized")
)

type ActorMembership struct {
	OrganizationID   string
	OrganizationName string
	OrganizationKind string
	Role             string
}

type Actor struct {
	UserID      string
	Email       string
	Name        string
	Memberships []ActorMembership
}

type Client interface {
	GetActor(ctx context.Context, bearerToken string) (Actor, error)
}

type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func NewClientFromEnv() Client {
	baseURL := strings.TrimSpace(os.Getenv("IAM_UPSTREAM"))
	if baseURL == "" {
		return nil
	}
	return NewClient(baseURL)
}

func (c *HTTPClient) GetActor(ctx context.Context, bearerToken string) (Actor, error) {
	if c == nil || c.baseURL == "" {
		return Actor{}, ErrNotConfigured
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/me", nil)
	if err != nil {
		return Actor{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))

	res, err := c.httpClient.Do(req)
	if err != nil {
		return Actor{}, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized {
		return Actor{}, ErrUnauthorized
	}
	if res.StatusCode >= http.StatusBadRequest {
		return Actor{}, fmt.Errorf("iam me status %d", res.StatusCode)
	}

	var payload struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"user"`
		Memberships []struct {
			Role         string `json:"role"`
			Organization struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Kind string `json:"kind"`
			} `json:"organization"`
		} `json:"memberships"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return Actor{}, err
	}

	actor := Actor{
		UserID: payload.User.ID,
		Email:  payload.User.Email,
		Name:   payload.User.Name,
	}
	for _, membership := range payload.Memberships {
		actor.Memberships = append(actor.Memberships, ActorMembership{
			OrganizationID:   membership.Organization.ID,
			OrganizationName: membership.Organization.Name,
			OrganizationKind: membership.Organization.Kind,
			Role:             membership.Role,
		})
	}

	return actor, nil
}
