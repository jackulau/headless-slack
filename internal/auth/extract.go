package auth

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ExtractXOXC fetches https://<team>.slack.com/ with the given xoxd cookie
// and parses the api_token from the boot_data block in the returned HTML.
//
// This is the same path the web app uses on first load. Slack's HTML contains
// a line like:
//
//	"api_token":"xoxc-1234...-5678"
//
// We extract that with a regex. No JS execution required.
func ExtractXOXC(team, xoxd string) (string, error) {
	if team == "" || xoxd == "" {
		return "", errors.New("team and xoxd both required")
	}
	u := fmt.Sprintf("https://%s.slack.com/", team)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}
	// Chrome-ish UA — Slack rejects obvious bots on this endpoint.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.AddCookie(&http.Cookie{Name: "d", Value: xoxd})

	client := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Follow up to 5 redirects, but preserve cookies.
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	return parseXOXCFromHTML(string(body))
}

var apiTokenRE = regexp.MustCompile(`"api_token"\s*:\s*"(xoxc-[^"]+)"`)

func parseXOXCFromHTML(html string) (string, error) {
	m := apiTokenRE.FindStringSubmatch(html)
	if len(m) < 2 {
		return "", errors.New("no xoxc api_token found in HTML (session may be expired)")
	}
	return m[1], nil
}

// TeamFromURL extracts the subdomain from a Slack workspace URL like
// "https://myco.slack.com" or "myco.slack.com" or just "myco".
func TeamFromURL(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", errors.New("empty team URL")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	// Bare name (no dot, no scheme, no slash) → treat as team subdomain.
	if !strings.ContainsAny(input, ".:/") {
		return input, nil
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("parse team URL %q: %w", input, err)
	}
	host := u.Host
	if i := strings.Index(host, ".slack.com"); i > 0 {
		sub := host[:i]
		// Reserved subdomains are not workspaces.
		switch sub {
		case "app", "api", "www", "files", "edgeapi":
			return "", fmt.Errorf("subdomain %q is not a workspace (use your <team>.slack.com URL)", sub)
		}
		return sub, nil
	}
	return "", fmt.Errorf("hostname %q is not a *.slack.com workspace", host)
}
