package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jacklau/headless-slack/internal/rtm"
)

func runWatch(filterChannel string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	s, err := open(ctx)
	if err != nil {
		return err
	}
	defer s.close()

	var filterID string
	if filterChannel != "" {
		id, err := resolveChannel(ctx, s, filterChannel)
		if err != nil {
			return err
		}
		filterID = id
	}

	bus := rtm.NewBus()
	sub := bus.Subscribe(64)
	defer bus.Unsubscribe(sub)

	client := rtm.NewClient(s.Client, bus)
	go func() { _ = client.Run(ctx) }()

	fmt.Fprintln(os.Stderr, "Connected — streaming events. Ctrl-C to stop.")
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-sub:
			if filterID != "" && ev.Channel != filterID {
				continue
			}
			renderEvent(ev)
		}
	}
}

func renderEvent(ev rtm.Event) {
	switch ev.Type {
	case rtm.EventMessage:
		fmt.Printf("[%s] %s %s: %s\n", formatTS(ev.TS), ev.Channel, ev.User, oneLine(ev.Text))
	case rtm.EventReactionAdded:
		fmt.Printf("[%s] %s +reaction by %s\n", formatTS(ev.TS), ev.Channel, ev.User)
	case rtm.EventUserTyping:
		// Skip — too noisy.
	case rtm.EventHello:
		fmt.Fprintln(os.Stderr, "(rtm: hello)")
	case rtm.EventGoodbye:
		fmt.Fprintln(os.Stderr, "(rtm: goodbye — will reconnect)")
	default:
		// Render unknown types minimally.
		fmt.Printf("(%s) %s\n", ev.Type, ev.Channel)
	}
}
