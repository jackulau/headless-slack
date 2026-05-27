package api

import (
	"context"
	"net/url"
)

// ChatPostMessage posts a message as the authenticated user.
//
// channel may be a channel ID (C…), group ID (G…), DM ID (D…), or — for
// convenience — a #channel-name string; Slack accepts the latter and resolves
// it server-side.
//
// Returns (channel, ts) of the posted message.
func (c *Client) ChatPostMessage(ctx context.Context, channel, text string) (string, string, error) {
	args := url.Values{}
	args.Set("channel", channel)
	args.Set("text", text)
	args.Set("as_user", "true")
	args.Set("link_names", "true")
	var resp struct {
		Channel string  `json:"channel"`
		TS      string  `json:"ts"`
		Message Message `json:"message"`
	}
	if err := c.Call(ctx, "chat.postMessage", args, &resp); err != nil {
		return "", "", err
	}
	return resp.Channel, resp.TS, nil
}

// ChatUpdate edits a previously posted message.
func (c *Client) ChatUpdate(ctx context.Context, channel, ts, text string) error {
	args := url.Values{}
	args.Set("channel", channel)
	args.Set("ts", ts)
	args.Set("text", text)
	return c.Call(ctx, "chat.update", args, nil)
}

// ChatDelete deletes a previously posted message.
func (c *Client) ChatDelete(ctx context.Context, channel, ts string) error {
	args := url.Values{}
	args.Set("channel", channel)
	args.Set("ts", ts)
	return c.Call(ctx, "chat.delete", args, nil)
}
