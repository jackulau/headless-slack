package api

import (
	"context"
	"net/url"
	"strconv"
)

// ConversationsList lists conversations visible to the calling user. Pass
// empty cursor for the first page.
func (c *Client) ConversationsList(ctx context.Context, types string, cursor string, limit int) ([]Channel, string, error) {
	if types == "" {
		types = "public_channel,private_channel,mpim,im"
	}
	if limit <= 0 {
		limit = 200
	}
	args := url.Values{}
	args.Set("types", types)
	args.Set("limit", strconv.Itoa(limit))
	args.Set("exclude_archived", "true")
	if cursor != "" {
		args.Set("cursor", cursor)
	}
	var resp struct {
		Channels         []Channel `json:"channels"`
		ResponseMetadata Cursor    `json:"response_metadata"`
	}
	if err := c.Call(ctx, "conversations.list", args, &resp); err != nil {
		return nil, "", err
	}
	return resp.Channels, resp.ResponseMetadata.NextCursor, nil
}

// ConversationsHistory returns recent messages in a channel.
func (c *Client) ConversationsHistory(ctx context.Context, channel string, limit int, oldest, latest, cursor string) ([]Message, string, error) {
	if limit <= 0 {
		limit = 50
	}
	args := url.Values{}
	args.Set("channel", channel)
	args.Set("limit", strconv.Itoa(limit))
	if oldest != "" {
		args.Set("oldest", oldest)
	}
	if latest != "" {
		args.Set("latest", latest)
	}
	if cursor != "" {
		args.Set("cursor", cursor)
	}
	var resp struct {
		Messages         []Message `json:"messages"`
		HasMore          bool      `json:"has_more"`
		ResponseMetadata Cursor    `json:"response_metadata"`
	}
	if err := c.Call(ctx, "conversations.history", args, &resp); err != nil {
		return nil, "", err
	}
	return resp.Messages, resp.ResponseMetadata.NextCursor, nil
}

// ConversationsReplies returns a thread.
func (c *Client) ConversationsReplies(ctx context.Context, channel, ts string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 100
	}
	args := url.Values{}
	args.Set("channel", channel)
	args.Set("ts", ts)
	args.Set("limit", strconv.Itoa(limit))
	var resp struct {
		Messages []Message `json:"messages"`
	}
	if err := c.Call(ctx, "conversations.replies", args, &resp); err != nil {
		return nil, err
	}
	return resp.Messages, nil
}

// ConversationsOpen opens a DM (or MPIM) and returns its channel ID.
func (c *Client) ConversationsOpen(ctx context.Context, userIDs ...string) (string, error) {
	args := url.Values{}
	args.Set("users", joinComma(userIDs))
	args.Set("return_im", "true")
	var resp struct {
		Channel Channel `json:"channel"`
	}
	if err := c.Call(ctx, "conversations.open", args, &resp); err != nil {
		return "", err
	}
	return resp.Channel.ID, nil
}

// ConversationsMark sets the read cursor for a channel.
func (c *Client) ConversationsMark(ctx context.Context, channel, ts string) error {
	args := url.Values{}
	args.Set("channel", channel)
	args.Set("ts", ts)
	return c.Call(ctx, "conversations.mark", args, nil)
}

func joinComma(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}
