package rtm

import (
	"encoding/json"
)

// Event is a parsed RTM event. Type is always present; the original bytes
// stay in Raw so consumers can decode subtype-specific fields without a
// second pass on the wire.
type Event struct {
	Type     string          `json:"type"`
	Subtype  string          `json:"subtype,omitempty"`
	Channel  string          `json:"channel,omitempty"`
	User     string          `json:"user,omitempty"`
	Text     string          `json:"text,omitempty"`
	TS       string          `json:"ts,omitempty"`
	ThreadTS string          `json:"thread_ts,omitempty"`
	Raw      json.RawMessage `json:"-"`
}

// Common type constants.
const (
	EventHello          = "hello"
	EventGoodbye        = "goodbye"
	EventMessage        = "message"
	EventReactionAdded  = "reaction_added"
	EventReactionRemove = "reaction_removed"
	EventChannelMarked  = "channel_marked"
	EventUserTyping     = "user_typing"
	EventPresenceChange = "presence_change"
	EventPong           = "pong"
	EventTeamJoin       = "team_join"
	EventChannelCreated = "channel_created"
)

// Decode parses an RTM frame.
func Decode(b []byte) (Event, error) {
	var ev Event
	if err := json.Unmarshal(b, &ev); err != nil {
		return Event{}, err
	}
	ev.Raw = append(json.RawMessage(nil), b...)
	return ev, nil
}
