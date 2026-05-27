package cli

import (
	"context"
	"fmt"
	"strings"
)

func runSend(channel string, msgParts []string) error {
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
	text := strings.Join(msgParts, " ")
	_, ts, err := s.Client.ChatPostMessage(ctx, chID, text)
	if err != nil {
		return err
	}
	fmt.Println(ts)
	return nil
}

func runDM(userInput string, msgParts []string) error {
	ctx := context.Background()
	s, err := open(ctx)
	if err != nil {
		return err
	}
	defer s.close()

	u, err := resolveUser(ctx, s, userInput)
	if err != nil {
		return err
	}
	chID, err := s.Client.ConversationsOpen(ctx, u.ID)
	if err != nil {
		return err
	}
	text := strings.Join(msgParts, " ")
	_, ts, err := s.Client.ChatPostMessage(ctx, chID, text)
	if err != nil {
		return err
	}
	fmt.Println(ts)
	return nil
}
