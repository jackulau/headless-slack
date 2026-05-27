// Package mock provides an in-process Slack-shaped server for tests.
//
// It implements just enough of the Slack Web API to exercise the slk client:
// conversations.list, conversations.history, chat.postMessage, users.list,
// users.info, conversations.open, rtm.connect (paired with a tiny WS server).
package mock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/jacklau/headless-slack/internal/api"
)

// Server is a mock Slack workspace. Goroutine-safe.
type Server struct {
	HTTP   *httptest.Server
	WS     *httptest.Server
	WSConn *websocket.Upgrader

	mu       sync.Mutex
	channels []api.Channel
	users    []api.User
	messages map[string][]api.Message // channel ID → newest first
}

// New returns a started Server with seed data.
func New() *Server {
	s := &Server{
		channels: []api.Channel{
			{ID: "C1", Name: "general", IsChannel: true, IsMember: true, NumMembers: 3},
			{ID: "C2", Name: "random", IsChannel: true, IsMember: true, NumMembers: 3},
			{ID: "D1", IsIM: true, User: "U2"},
		},
		users: []api.User{
			{ID: "U1", Name: "alice", RealName: "Alice A"},
			{ID: "U2", Name: "bob", RealName: "Bob B"},
		},
		messages: map[string][]api.Message{
			"C1": {
				{Type: "message", User: "U1", Text: "hello world", TS: "1700000000.000100"},
				{Type: "message", User: "U2", Text: "hi back", TS: "1700000001.000100"},
			},
		},
	}
	s.WSConn = &websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	// WS server first so we can embed its URL in rtm.connect.
	s.WS = httptest.NewServer(http.HandlerFunc(s.handleWS))
	s.HTTP = httptest.NewServer(http.HandlerFunc(s.handleHTTP))
	return s
}

func (s *Server) Close() {
	s.HTTP.Close()
	s.WS.Close()
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/api/")
	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "conversations.list":
		s.mu.Lock()
		defer s.mu.Unlock()
		respond(w, map[string]any{
			"ok":                true,
			"channels":          s.channels,
			"response_metadata": map[string]string{"next_cursor": ""},
		})
	case "conversations.history":
		_ = r.ParseForm()
		ch := r.Form.Get("channel")
		s.mu.Lock()
		defer s.mu.Unlock()
		respond(w, map[string]any{
			"ok":       true,
			"messages": s.messages[ch],
		})
	case "chat.postMessage":
		_ = r.ParseForm()
		ch := r.Form.Get("channel")
		text := r.Form.Get("text")
		ts := fmt.Sprintf("17%010d.000100", len(s.messages[ch])+1)
		s.mu.Lock()
		s.messages[ch] = append(s.messages[ch], api.Message{Type: "message", User: "U1", Text: text, TS: ts})
		s.mu.Unlock()
		respond(w, map[string]any{"ok": true, "channel": ch, "ts": ts})
	case "users.list":
		s.mu.Lock()
		defer s.mu.Unlock()
		respond(w, map[string]any{
			"ok":                true,
			"members":           s.users,
			"response_metadata": map[string]string{"next_cursor": ""},
		})
	case "users.info":
		_ = r.ParseForm()
		uid := r.Form.Get("user")
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, u := range s.users {
			if u.ID == uid {
				respond(w, map[string]any{"ok": true, "user": u})
				return
			}
		}
		respond(w, map[string]any{"ok": false, "error": "user_not_found"})
	case "conversations.open":
		respond(w, map[string]any{"ok": true, "channel": map[string]any{"id": "D1", "is_im": true, "user": "U2"}})
	case "conversations.mark":
		respond(w, map[string]any{"ok": true})
	case "rtm.connect":
		wsURL := "ws" + strings.TrimPrefix(s.WS.URL, "http")
		respond(w, map[string]any{
			"ok":   true,
			"url":  wsURL,
			"self": map[string]string{"id": "U1", "name": "alice"},
			"team": map[string]string{"id": "T1", "name": "mock", "domain": "mock"},
		})
	default:
		respond(w, map[string]any{"ok": false, "error": "unknown_method"})
	}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.WSConn.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	send := func(v any) {
		b, _ := json.Marshal(v)
		_ = conn.WriteMessage(websocket.TextMessage, b)
	}
	send(map[string]string{"type": "hello"})
	send(map[string]any{
		"type":    "message",
		"channel": "C1",
		"user":    "U2",
		"text":    "live event",
		"ts":      "1700000100.000100",
	})
	// Wait for client to close or send anything.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func respond(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}

// Client returns an api.Client pointed at this mock server.
func (s *Server) Client() *api.Client {
	return &api.Client{
		Auth: &api.AuthClient{
			Inner: s.HTTP.Client(),
			XOXC:  "xoxc-mock",
			XOXD:  "xoxd-mock",
		},
		Team:    "mock",
		BaseURL: s.HTTP.URL,
	}
}
