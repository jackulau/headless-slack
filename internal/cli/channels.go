package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/jacklau/headless-slack/internal/api"
)

func runChannels() error {
	ctx := context.Background()
	s, err := open(ctx)
	if err != nil {
		return err
	}
	defer s.close()

	all, err := fetchAllChannels(ctx, s.Client)
	if err != nil {
		return err
	}
	// Cache for later commands.
	for _, c := range all {
		_ = s.Store.PutChannel(ctx, c)
	}

	sort.Slice(all, func(i, j int) bool {
		ki, kj := kindOrder(all[i]), kindOrder(all[j])
		if ki != kj {
			return ki < kj
		}
		return all[i].Name < all[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tKIND\tNAME\tMEMBERS\tTOPIC")
	for _, c := range all {
		name := c.Name
		if c.IsIM {
			name = "(dm)"
		}
		topic := strings.SplitN(c.Topic.Value, "\n", 2)[0]
		if len(topic) > 60 {
			topic = topic[:60] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", c.ID, kindLabel(c), name, c.NumMembers, topic)
	}
	return w.Flush()
}

func fetchAllChannels(ctx context.Context, c *api.Client) ([]api.Channel, error) {
	var all []api.Channel
	cursor := ""
	for {
		page, next, err := c.ConversationsList(ctx, "", cursor, 200)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if next == "" {
			return all, nil
		}
		cursor = next
	}
}

func kindLabel(c api.Channel) string {
	switch {
	case c.IsIM:
		return "im"
	case c.IsMpim:
		return "mpim"
	case c.IsPrivate, c.IsGroup:
		return "group"
	default:
		return "channel"
	}
}

func kindOrder(c api.Channel) int {
	switch {
	case c.IsChannel && !c.IsPrivate:
		return 0
	case c.IsPrivate, c.IsGroup:
		return 1
	case c.IsMpim:
		return 2
	case c.IsIM:
		return 3
	default:
		return 4
	}
}
