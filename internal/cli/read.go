package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jacklau/headless-slack/internal/api"
)

func runRead(channel string, last int) error {
	ctx := context.Background()
	s, err := open(ctx)
	if err != nil {
		return err
	}
	defer s.close()

	chID, err := resolveChannel(ctx, s, channel)
	if err != nil {
		return err
	}

	msgs, _, err := s.Client.ConversationsHistory(ctx, chID, last, "", "", "")
	if err != nil {
		return err
	}
	// API returns newest-first; flip to chronological for human reading.
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].TS < msgs[j].TS })
	// Cache for later.
	for _, m := range msgs {
		_ = s.Store.PutMessage(ctx, chID, m)
	}

	// Resolve user IDs to names.
	userNames := map[string]string{}
	for _, m := range msgs {
		if m.User != "" {
			userNames[m.User] = m.User
		}
	}
	for id := range userNames {
		if u, err := s.Store.GetUser(ctx, id); err == nil {
			userNames[id] = displayName(u)
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	for _, m := range msgs {
		who := userNames[m.User]
		if who == "" {
			who = m.User
		}
		if m.BotID != "" && who == "" {
			who = m.BotID
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", formatTS(m.TS), trunc(who, 18), oneLine(m.Text))
	}
	return w.Flush()
}

func formatTS(ts string) string {
	if ts == "" {
		return ""
	}
	// Slack ts is "<unix>.<micro>"
	dot := strings.IndexByte(ts, '.')
	sec := ts
	if dot >= 0 {
		sec = ts[:dot]
	}
	n, err := strconv.ParseInt(sec, 10, 64)
	if err != nil {
		return ts
	}
	return time.Unix(n, 0).Format("2006-01-02 15:04:05")
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func oneLine(s string) string {
	return strings.ReplaceAll(s, "\n", " ⏎ ")
}

func displayName(u api.User) string {
	if u.Profile.DisplayName != "" {
		return u.Profile.DisplayName
	}
	if u.RealName != "" {
		return u.RealName
	}
	if u.Name != "" {
		return u.Name
	}
	return u.ID
}
