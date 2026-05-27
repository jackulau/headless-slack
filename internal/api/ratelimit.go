package api

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// Tier is a Slack Web API rate-limit tier.
//
//	Tier1   ~  1 req/min   admin.users.*, etc.
//	Tier2   ~ 20 req/min   conversations.list, users.list
//	Tier3   ~ 50 req/min   conversations.history (xoxc/xoxp), users.info
//	Tier4   ~100 req/min   conversations.mark, reactions.add
//	TierPost ~ 1 req/sec  chat.postMessage (special, per-channel)
type Tier int

const (
	TierUnknown Tier = iota
	Tier1
	Tier2
	Tier3
	Tier4
	TierPost
)

// methodTier maps the Slack method (last path segment) to its tier.
// Defaults to Tier3 for unknown methods — Slack's docs say "most" sit there.
var methodTier = map[string]Tier{
	// Tier 2
	"conversations.list":    Tier2,
	"users.list":            Tier2,
	"users.conversations":   Tier2,
	"conversations.members": Tier2,
	// Tier 3
	"conversations.history": Tier3,
	"conversations.replies": Tier3,
	"conversations.info":    Tier3,
	"users.info":            Tier3,
	"client.userBoot":       Tier3,
	"client.boot":           Tier3,
	"rtm.connect":           Tier3,
	"search.messages":       Tier3,
	// Tier 4
	"conversations.mark": Tier4,
	"reactions.add":      Tier4,
	"reactions.remove":   Tier4,
	// Post (special)
	"chat.postMessage":   TierPost,
	"chat.update":        TierPost,
	"chat.delete":        TierPost,
	"chat.postEphemeral": TierPost,
}

// Limiter applies per-tier token-bucket rate limits. Bursts allow a small
// cushion above the steady-state rate; this mirrors what the desktop client
// gets in practice.
type Limiter struct {
	mu      sync.Mutex
	buckets map[Tier]*rate.Limiter
}

func NewLimiter() *Limiter {
	return &Limiter{
		buckets: map[Tier]*rate.Limiter{
			Tier1:    rate.NewLimiter(rate.Limit(1.0/60), 1),    //  1/min, burst 1
			Tier2:    rate.NewLimiter(rate.Limit(20.0/60), 5),   // 20/min, burst 5
			Tier3:    rate.NewLimiter(rate.Limit(50.0/60), 10),  // 50/min, burst 10
			Tier4:    rate.NewLimiter(rate.Limit(100.0/60), 20), // 100/min, burst 20
			TierPost: rate.NewLimiter(rate.Limit(1), 3),         //  1/sec, burst 3
		},
	}
}

// WaitFor blocks until the limiter for the given method's tier admits one
// request, or ctx is canceled.
func (l *Limiter) WaitFor(ctx context.Context, method string) error {
	t := TierOf(method)
	l.mu.Lock()
	b, ok := l.buckets[t]
	l.mu.Unlock()
	if !ok {
		return nil
	}
	return b.Wait(ctx)
}

// TierOf returns the tier of a Slack method by last path segment.
func TierOf(method string) Tier {
	if t, ok := methodTier[method]; ok {
		return t
	}
	return Tier3
}
