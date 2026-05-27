package api

import (
	"context"
	"net/url"
)

// RTMConnectResp is the subset of rtm.connect we care about.
type RTMConnectResp struct {
	URL  string `json:"url"`
	Self struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"self"`
	Team struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Domain string `json:"domain"`
	} `json:"team"`
}

// RTMConnect calls rtm.connect to obtain a fresh WebSocket URL.
//
// Despite official deprecation for new Slack apps, this endpoint still works
// for xoxc/xoxp user tokens in 2026. It is what wee-slack and matterircd use.
func (c *Client) RTMConnect(ctx context.Context) (RTMConnectResp, error) {
	args := url.Values{}
	args.Set("batch_presence_aware", "1")
	args.Set("presence_sub", "true")
	var out RTMConnectResp
	if err := c.Call(ctx, "rtm.connect", args, &out); err != nil {
		return RTMConnectResp{}, err
	}
	return out, nil
}

// ClientUserBootResp is the (heavily trimmed) shape of client.userBoot.
type ClientUserBootResp struct {
	Self User `json:"self"`
	Team struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Domain string `json:"domain"`
	} `json:"team"`
	Channels []Channel `json:"channels"`
	IMs      []Channel `json:"ims"`
}

// ClientUserBoot is a single-call bootstrap of the workspace state. Faster
// than calling conversations.list + users.list separately on startup.
func (c *Client) ClientUserBoot(ctx context.Context) (ClientUserBootResp, error) {
	args := url.Values{}
	args.Set("flannel_api_ver", "4")
	args.Set("build_version_ts", "0")
	var out ClientUserBootResp
	if err := c.Call(ctx, "client.userBoot", args, &out); err != nil {
		return ClientUserBootResp{}, err
	}
	return out, nil
}
