package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

func runSearch(parts []string) error {
	ctx := context.Background()
	q := strings.Join(parts, " ")
	if q == "" {
		return fmt.Errorf("query required")
	}
	s, err := open(ctx)
	if err != nil {
		return err
	}
	defer s.close()

	// Try server search first; fall back to local store.
	msgs, err := s.Client.SearchMessages(ctx, q, 1)
	if err != nil || len(msgs) == 0 {
		msgs, err = s.Store.SearchMessages(ctx, q, 50)
		if err != nil {
			return err
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	for _, m := range msgs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", formatTS(m.TS), m.Channel, m.User, oneLine(m.Text))
	}
	return w.Flush()
}
