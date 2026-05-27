package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
)

// DefaultUA is the User-Agent string Slack expects to see from a desktop
// Chrome client. It is paired with the utls Chrome ClientHello so the JA3
// fingerprint and the UA agree.
const DefaultUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// NewTransport returns an *http.Transport whose TLS handshake mimics Chrome
// via utls.HelloChrome_Auto.
//
// Slack runs an Anomaly Event Response (AER) layer that flags requests where
// the TLS ClientHello does not match the User-Agent. Using Go's default
// crypto/tls handshake is a fast way to get a session flagged. utls produces
// a ClientHello indistinguishable from real Chrome.
func NewTransport() *http.Transport {
	return &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialUTLS(ctx, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
}

func dialUTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 10 * time.Second}
	raw, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	conf := &utls.Config{ServerName: host, NextProtos: []string{"h2", "http/1.1"}}
	uc := utls.UClient(raw, conf, utls.HelloChrome_Auto)
	if err := uc.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return uc, nil
}

// Doer is implemented by *http.Client. Tests can substitute a stub.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// AuthClient wraps a Doer and injects xoxc + xoxd credentials on every request
// targeted at *.slack.com. It also applies a tier-aware rate limiter.
type AuthClient struct {
	Inner   Doer
	XOXC    string
	XOXD    string
	UA      string
	Limiter *Limiter

	// Optional override for tests — pin Origin header.
	Origin string
}

func (c *AuthClient) Do(req *http.Request) (*http.Response, error) {
	if c.XOXC == "" || c.XOXD == "" {
		return nil, errors.New("AuthClient: xoxc + xoxd required")
	}
	if isSlackHost(req.URL) {
		req.AddCookie(&http.Cookie{Name: "d", Value: c.XOXD})
		ua := c.UA
		if ua == "" {
			ua = DefaultUA
		}
		req.Header.Set("User-Agent", ua)
		if c.Origin != "" {
			req.Header.Set("Origin", c.Origin)
		}
	}
	if c.Limiter != nil {
		if err := c.Limiter.WaitFor(req.Context(), methodFromURL(req.URL)); err != nil {
			return nil, err
		}
	}
	inner := c.Inner
	if inner == nil {
		inner = http.DefaultClient
	}
	return inner.Do(req)
}

// AuthorizeForm sets the token form-field on a POST request body — Slack's
// Web API methods accept the token as a form value as well as a bearer header.
// Most Slack methods are application/x-www-form-urlencoded so this is the
// common path.
func (c *AuthClient) AuthorizeForm(form url.Values) {
	form.Set("token", c.XOXC)
}

func isSlackHost(u *url.URL) bool {
	if u == nil {
		return false
	}
	h := u.Hostname()
	return h == "slack.com" || strings.HasSuffix(h, ".slack.com")
}

func methodFromURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	// Slack methods live under /api/<methodName>
	p := u.Path
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return ""
}

// Stringify produces a one-line summary of a request useful for debug logs.
func Stringify(req *http.Request) string {
	if req == nil || req.URL == nil {
		return "<nil request>"
	}
	return fmt.Sprintf("%s %s", req.Method, req.URL)
}

// Compile-time check.
var _ Doer = (*AuthClient)(nil)
var _ Doer = (*http.Client)(nil)

// _ is a sink to silence unused imports during partial builds.
var _ = tls.VersionTLS13
