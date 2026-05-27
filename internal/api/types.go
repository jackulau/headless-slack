package api

// Channel describes a conversation (channel, group, IM, MPIM).
type Channel struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	IsChannel  bool   `json:"is_channel,omitempty"`
	IsGroup    bool   `json:"is_group,omitempty"`
	IsIM       bool   `json:"is_im,omitempty"`
	IsMpim     bool   `json:"is_mpim,omitempty"`
	IsPrivate  bool   `json:"is_private,omitempty"`
	IsArchived bool   `json:"is_archived,omitempty"`
	IsMember   bool   `json:"is_member,omitempty"`
	User       string `json:"user,omitempty"` // for IM: the other user's ID
	NumMembers int    `json:"num_members,omitempty"`
	Topic      struct {
		Value string `json:"value"`
	} `json:"topic,omitempty"`
	Purpose struct {
		Value string `json:"value"`
	} `json:"purpose,omitempty"`
}

// User describes a workspace member.
type User struct {
	ID       string `json:"id"`
	TeamID   string `json:"team_id,omitempty"`
	Name     string `json:"name,omitempty"` // legacy login name
	RealName string `json:"real_name,omitempty"`
	Deleted  bool   `json:"deleted,omitempty"`
	IsBot    bool   `json:"is_bot,omitempty"`
	Profile  struct {
		DisplayName string `json:"display_name"`
		RealName    string `json:"real_name"`
		Email       string `json:"email,omitempty"`
		Image48     string `json:"image_48,omitempty"`
	} `json:"profile,omitempty"`
}

// Message is a single chat message as returned by conversations.history /
// replies. Slack message shapes are huge; we keep the fields a CLI client
// actually renders.
type Message struct {
	Type       string `json:"type"`
	Subtype    string `json:"subtype,omitempty"`
	User       string `json:"user,omitempty"`
	BotID      string `json:"bot_id,omitempty"`
	Text       string `json:"text,omitempty"`
	TS         string `json:"ts"`
	ThreadTS   string `json:"thread_ts,omitempty"`
	ReplyCount int    `json:"reply_count,omitempty"`
	Reactions  []struct {
		Name  string   `json:"name"`
		Count int      `json:"count"`
		Users []string `json:"users"`
	} `json:"reactions,omitempty"`
	Edited *struct {
		User string `json:"user"`
		TS   string `json:"ts"`
	} `json:"edited,omitempty"`
	Channel string `json:"channel,omitempty"`
}

// Cursor is a Slack pagination cursor.
type Cursor struct {
	NextCursor string `json:"next_cursor"`
}
