package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
)

func runUsers() error {
	ctx := context.Background()
	s, err := open(ctx)
	if err != nil {
		return err
	}
	defer s.close()

	users, err := s.Client.UsersList(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		_ = s.Store.PutUser(ctx, u)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tREAL\tEMAIL")
	for _, u := range users {
		if u.Deleted {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.ID, u.Name, u.RealName, u.Profile.Email)
	}
	return w.Flush()
}
