package api

import (
	"context"
	"net/url"
	"strconv"
)

// UsersList returns all workspace users (paginated).
func (c *Client) UsersList(ctx context.Context) ([]User, error) {
	var all []User
	cursor := ""
	for {
		args := url.Values{}
		args.Set("limit", "200")
		if cursor != "" {
			args.Set("cursor", cursor)
		}
		var resp struct {
			Members          []User `json:"members"`
			ResponseMetadata Cursor `json:"response_metadata"`
		}
		if err := c.Call(ctx, "users.list", args, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Members...)
		if resp.ResponseMetadata.NextCursor == "" {
			return all, nil
		}
		cursor = resp.ResponseMetadata.NextCursor
	}
}

// UsersInfo returns details for a single user.
func (c *Client) UsersInfo(ctx context.Context, userID string) (User, error) {
	args := url.Values{}
	args.Set("user", userID)
	var resp struct {
		User User `json:"user"`
	}
	if err := c.Call(ctx, "users.info", args, &resp); err != nil {
		return User{}, err
	}
	return resp.User, nil
}

// SearchMessages runs a server-side search.
func (c *Client) SearchMessages(ctx context.Context, query string, page int) ([]Message, error) {
	args := url.Values{}
	args.Set("query", query)
	args.Set("count", "20")
	args.Set("sort", "timestamp")
	args.Set("sort_dir", "desc")
	if page > 0 {
		args.Set("page", strconv.Itoa(page))
	}
	var resp struct {
		Messages struct {
			Matches []Message `json:"matches"`
		} `json:"messages"`
	}
	if err := c.Call(ctx, "search.messages", args, &resp); err != nil {
		return nil, err
	}
	return resp.Messages.Matches, nil
}
