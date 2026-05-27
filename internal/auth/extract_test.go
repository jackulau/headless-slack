package auth

import (
	"testing"
)

func TestParseXOXCFromHTML(t *testing.T) {
	cases := []struct {
		name    string
		html    string
		want    string
		wantErr bool
	}{
		{
			name: "boot_data block",
			html: `<script>boot_data = {"team_id":"T123","api_token":"xoxc-1-abc","other":"x"};</script>`,
			want: "xoxc-1-abc",
		},
		{
			name: "spaces around colon",
			html: `"api_token" : "xoxc-spaced-token"`,
			want: "xoxc-spaced-token",
		},
		{
			name:    "missing token",
			html:    `<html>no token here</html>`,
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseXOXCFromHTML(c.html)
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
			if got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestTeamFromURL(t *testing.T) {
	cases := map[string]string{
		"https://myco.slack.com":         "myco",
		"https://myco.slack.com/":        "myco",
		"http://myco.slack.com/messages": "myco",
		"myco.slack.com":                 "myco",
		"myco":                           "myco",
		"app.slack.com":                  "",
	}
	for in, want := range cases {
		got, err := TeamFromURL(in)
		if want == "" {
			if err == nil {
				t.Errorf("input %q: expected error, got %q", in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("input %q: unexpected err: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("input %q: got %q want %q", in, got, want)
		}
	}
}

func TestTokensValidate(t *testing.T) {
	cases := []struct {
		name string
		t    Tokens
		ok   bool
	}{
		{"valid", Tokens{Team: "x", XOXC: "xoxc-a", XOXD: "xoxd-b"}, true},
		{"bad xoxc", Tokens{Team: "x", XOXC: "abc", XOXD: "xoxd-b"}, false},
		{"bad xoxd", Tokens{Team: "x", XOXC: "xoxc-a", XOXD: "abc"}, false},
		{"empty team", Tokens{Team: "", XOXC: "xoxc-a", XOXD: "xoxd-b"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.t.Validate()
			if (err == nil) != c.ok {
				t.Fatalf("ok=%v err=%v", c.ok, err)
			}
		})
	}
}
