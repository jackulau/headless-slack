package rtm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jacklau/headless-slack/internal/api"
)

// Connector is the subset of *api.Client we need (used so tests can stub).
type Connector interface {
	RTMConnect(ctx context.Context) (api.RTMConnectResp, error)
}

// Client runs the RTM WebSocket loop and republishes parsed events on Bus.
type Client struct {
	API    Connector
	Bus    *Bus
	Dialer *websocket.Dialer // nil → use default
	Logger *log.Logger

	// PingInterval is the gap between client → server pings.
	// Slack closes the socket after ~30s of silence, so 20s is the safe default.
	PingInterval time.Duration

	connected atomic.Bool
	msgID     atomic.Int64
}

// NewClient constructs a Client with sensible defaults.
func NewClient(a Connector, bus *Bus) *Client {
	return &Client{
		API:          a,
		Bus:          bus,
		PingInterval: 20 * time.Second,
		Dialer:       websocket.DefaultDialer,
	}
}

func (c *Client) logf(f string, a ...any) {
	if c.Logger != nil {
		c.Logger.Printf(f, a...)
	}
}

// Connected reports whether the client currently has an open WS session.
func (c *Client) Connected() bool { return c.connected.Load() }

// Run blocks until ctx is canceled. It reconnects on disconnect with capped
// exponential backoff (1s → 60s) plus jitter.
func (c *Client) Run(ctx context.Context) error {
	backoff := time.Second
	for ctx.Err() == nil {
		if err := c.runOnce(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			c.logf("rtm: %v — reconnecting in %s", err, backoff)
		}
		// Backoff sleep, but exit fast on ctx cancel.
		t := time.NewTimer(backoff + jitter(backoff/2))
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}
		backoff *= 2
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
	}
	return nil
}

func (c *Client) runOnce(ctx context.Context) error {
	resp, err := c.API.RTMConnect(ctx)
	if err != nil {
		return fmt.Errorf("rtm.connect: %w", err)
	}
	if resp.URL == "" {
		return errors.New("rtm.connect returned empty url")
	}
	d := c.Dialer
	if d == nil {
		d = websocket.DefaultDialer
	}
	conn, _, err := d.DialContext(ctx, resp.URL, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", resp.URL, err)
	}
	defer conn.Close()
	c.connected.Store(true)
	defer c.connected.Store(false)

	c.logf("rtm: connected as %s (%s)", resp.Self.Name, resp.Self.ID)

	pingCtx, cancelPing := context.WithCancel(ctx)
	defer cancelPing()
	go c.pingLoop(pingCtx, conn)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		ev, perr := Decode(msg)
		if perr != nil {
			c.logf("rtm: decode: %v", perr)
			continue
		}
		c.Bus.Publish(ev)
		if ev.Type == EventGoodbye {
			return errors.New("server said goodbye")
		}
	}
}

func (c *Client) pingLoop(ctx context.Context, conn *websocket.Conn) {
	t := time.NewTicker(c.PingInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			id := c.msgID.Add(1)
			payload, _ := json.Marshal(map[string]any{"id": id, "type": "ping"})
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		}
	}
}

func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(d)))
}
