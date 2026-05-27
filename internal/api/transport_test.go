package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAuthClient_AddsCookieAndUA(t *testing.T) {
	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(context.Background())
		w.WriteHeader(200)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	// httptest gives us 127.0.0.1 — isSlackHost won't match. Force-rewrite host.
	u, _ := url.Parse(srv.URL + "/api/conversations.list")
	// We can't actually rewrite Host without DNS, but we CAN test the header
	// injection logic by pointing at a URL whose hostname ends in .slack.com
	// via a custom RoundTripper that maps host → test server.

	srvURL, _ := url.Parse(srv.URL)
	mapping := map[string]string{"slack.com": srvURL.Host, "myco.slack.com": srvURL.Host}
	tr := &remapTransport{base: http.DefaultTransport, host: mapping}

	c := &AuthClient{
		Inner: &http.Client{Transport: tr},
		XOXC:  "xoxc-test",
		XOXD:  "xoxd-test",
		UA:    "MyUA/1.0",
	}

	req, _ := http.NewRequest("POST", "https://myco.slack.com/api/conversations.list", strings.NewReader(""))
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if got.Header.Get("User-Agent") != "MyUA/1.0" {
		t.Errorf("UA = %q, want MyUA/1.0", got.Header.Get("User-Agent"))
	}
	ck, err := got.Cookie("d")
	if err != nil {
		t.Fatalf("no d cookie: %v", err)
	}
	if ck.Value != "xoxd-test" {
		t.Errorf("d cookie = %q, want xoxd-test", ck.Value)
	}
	_ = u
}

func TestAuthClient_RejectsMissingCreds(t *testing.T) {
	c := &AuthClient{XOXC: "", XOXD: "xoxd-x"}
	req, _ := http.NewRequest("GET", "https://myco.slack.com/", nil)
	if _, err := c.Do(req); err == nil {
		t.Fatal("expected error on missing xoxc")
	}
}

func TestAuthClient_SkipsCookieForNonSlackHost(t *testing.T) {
	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(context.Background())
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := &AuthClient{
		Inner: srv.Client(),
		XOXC:  "xoxc-x",
		XOXD:  "xoxd-x",
	}
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if _, err := got.Cookie("d"); err == nil {
		t.Error("non-slack host should not receive the d cookie")
	}
}

func TestTierOf(t *testing.T) {
	cases := map[string]Tier{
		"conversations.list":    Tier2,
		"conversations.history": Tier3,
		"chat.postMessage":      TierPost,
		"conversations.mark":    Tier4,
		"asdfqwer":              Tier3, // default
	}
	for m, want := range cases {
		if got := TierOf(m); got != want {
			t.Errorf("%s: got %d want %d", m, got, want)
		}
	}
}

func TestLimiter_RespectsTier(t *testing.T) {
	l := NewLimiter()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// Tier4 burst is 20 — 5 calls should all admit immediately.
	for i := 0; i < 5; i++ {
		if err := l.WaitFor(ctx, "conversations.mark"); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
}

func TestMethodFromURL(t *testing.T) {
	u, _ := url.Parse("https://myco.slack.com/api/conversations.history?cursor=abc")
	if got := methodFromURL(u); got != "conversations.history" {
		t.Errorf("got %q want conversations.history", got)
	}
}

// remapTransport rewrites the request's Host:port to the test-server's so we
// can exercise host-aware logic (cookie injection) without DNS games.
type remapTransport struct {
	base http.RoundTripper
	host map[string]string // logical host → test host
}

func (r *remapTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if to, ok := r.host[req.URL.Hostname()]; ok {
		req = req.Clone(req.Context())
		req.URL.Scheme = "http"
		req.URL.Host = to
		req.Host = to
	}
	return r.base.RoundTrip(req)
}
