package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client is a typed Slack Web API client over an AuthClient.
type Client struct {
	Auth    *AuthClient
	Team    string // workspace subdomain, e.g. "myco"
	BaseURL string // override for tests; default https://<team>.slack.com
}

// New constructs a Client with default transport + rate limiter.
func New(team, xoxc, xoxd string) *Client {
	return &Client{
		Auth: &AuthClient{
			Inner:   &http.Client{Transport: NewTransport()},
			XOXC:    xoxc,
			XOXD:    xoxd,
			UA:      DefaultUA,
			Limiter: NewLimiter(),
			Origin:  "https://app.slack.com",
		},
		Team: team,
	}
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://" + c.Team + ".slack.com"
}

// SlackError is returned when the Web API replies with ok:false.
type SlackError struct {
	Method  string
	APIErr  string
	Warning string
}

func (e *SlackError) Error() string {
	if e.Warning != "" {
		return fmt.Sprintf("slack %s: %s (warning: %s)", e.Method, e.APIErr, e.Warning)
	}
	return fmt.Sprintf("slack %s: %s", e.Method, e.APIErr)
}

// Call POSTs form-encoded args to /api/<method> and JSON-decodes the result.
//
// The token is added to the form automatically. dst should be a pointer to the
// expected response struct; the wrapper checks the "ok" field and surfaces
// errors as *SlackError.
func (c *Client) Call(ctx context.Context, method string, args url.Values, dst any) error {
	if args == nil {
		args = url.Values{}
	}
	c.Auth.AuthorizeForm(args)

	endpoint := c.baseURL() + "/api/" + method
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(args.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	resp, err := c.Auth.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("%s: read body: %w", method, err)
	}

	// Slack always returns JSON with at least {"ok": bool, "error": "..."}.
	var head struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		Warning string `json:"warning"`
	}
	if err := json.Unmarshal(body, &head); err != nil {
		return fmt.Errorf("%s: decode head: %w (body %d bytes)", method, err, len(body))
	}
	if !head.OK {
		return &SlackError{Method: method, APIErr: head.Error, Warning: head.Warning}
	}
	if dst != nil {
		if err := json.Unmarshal(body, dst); err != nil {
			return fmt.Errorf("%s: decode body: %w", method, err)
		}
	}
	return nil
}

// GetJSON sends a GET to <baseURL><path> with the auth cookie attached and
// JSON-decodes the response. Used for endpoints that don't accept form POSTs
// (e.g. some edge API routes).
func (c *Client) GetJSON(ctx context.Context, fullURL string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.Auth.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("GET %s: %d", fullURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if dst != nil {
		return json.Unmarshal(body, dst)
	}
	return nil
}

// IsAuthError reports whether err is a Slack invalid_auth or token_revoked.
func IsAuthError(err error) bool {
	var se *SlackError
	if errors.As(err, &se) {
		switch se.APIErr {
		case "invalid_auth", "not_authed", "token_revoked", "token_expired", "account_inactive":
			return true
		}
	}
	return false
}
