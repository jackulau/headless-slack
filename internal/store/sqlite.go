package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/jacklau/headless-slack/internal/api"
)

//go:embed schema.sql
var schemaSQL string

// Store is the local message+user+channel cache.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) a SQLite database at path with WAL mode.
// Pass ":memory:" for a transient store useful in tests.
func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
	}
	dsn := path + "?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL&_foreign_keys=on"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite single-writer; single conn avoids "database is locked"
	if _, err := db.ExecContext(context.Background(), schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// PutUser upserts a user.
func (s *Store) PutUser(ctx context.Context, u api.User) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users(id, name, real_name, display_name, email, image_48, is_bot, deleted, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
		    name=excluded.name, real_name=excluded.real_name,
		    display_name=excluded.display_name, email=excluded.email,
		    image_48=excluded.image_48, is_bot=excluded.is_bot,
		    deleted=excluded.deleted, updated_at=excluded.updated_at`,
		u.ID, u.Name, u.RealName, u.Profile.DisplayName, u.Profile.Email, u.Profile.Image48,
		boolToInt(u.IsBot), boolToInt(u.Deleted), time.Now().Unix())
	return err
}

// GetUser returns a single user or sql.ErrNoRows.
func (s *Store) GetUser(ctx context.Context, id string) (api.User, error) {
	var u api.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, real_name, display_name, email, image_48, is_bot, deleted
		   FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Name, &u.RealName, &u.Profile.DisplayName, &u.Profile.Email, &u.Profile.Image48,
			boolPtr(&u.IsBot), boolPtr(&u.Deleted))
	return u, err
}

// PutChannel upserts a channel.
func (s *Store) PutChannel(ctx context.Context, c api.Channel) error {
	kind := channelKind(c)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO channels(id, name, kind, is_private, is_archived, is_member, other_user, topic, purpose, num_members, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
		    name=excluded.name, kind=excluded.kind, is_private=excluded.is_private,
		    is_archived=excluded.is_archived, is_member=excluded.is_member,
		    other_user=excluded.other_user, topic=excluded.topic, purpose=excluded.purpose,
		    num_members=excluded.num_members, updated_at=excluded.updated_at`,
		c.ID, c.Name, kind, boolToInt(c.IsPrivate), boolToInt(c.IsArchived), boolToInt(c.IsMember),
		c.User, c.Topic.Value, c.Purpose.Value, c.NumMembers, time.Now().Unix())
	return err
}

// ListChannels returns cached channels filtered by kind (empty = all).
func (s *Store) ListChannels(ctx context.Context, kind string) ([]api.Channel, error) {
	q := `SELECT id, name, kind, is_private, is_archived, is_member, other_user, topic, purpose, num_members
	      FROM channels WHERE is_archived = 0`
	args := []any{}
	if kind != "" {
		q += " AND kind = ?"
		args = append(args, kind)
	}
	q += " ORDER BY kind, name"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.Channel
	for rows.Next() {
		var c api.Channel
		var k string
		if err := rows.Scan(&c.ID, &c.Name, &k, boolPtr(&c.IsPrivate), boolPtr(&c.IsArchived),
			boolPtr(&c.IsMember), &c.User, &c.Topic.Value, &c.Purpose.Value, &c.NumMembers); err != nil {
			return nil, err
		}
		applyKind(&c, k)
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindChannelByName looks up a channel by exact name (no leading #).
func (s *Store) FindChannelByName(ctx context.Context, name string) (api.Channel, error) {
	var c api.Channel
	var k string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, kind, is_private, is_archived, is_member, other_user, topic, purpose, num_members
		   FROM channels WHERE name = ? LIMIT 1`, name).
		Scan(&c.ID, &c.Name, &k, boolPtr(&c.IsPrivate), boolPtr(&c.IsArchived),
			boolPtr(&c.IsMember), &c.User, &c.Topic.Value, &c.Purpose.Value, &c.NumMembers)
	if err != nil {
		return api.Channel{}, err
	}
	applyKind(&c, k)
	return c, nil
}

// PutMessage upserts a message.
func (s *Store) PutMessage(ctx context.Context, channel string, m api.Message) error {
	raw, _ := json.Marshal(m)
	var editedTS string
	if m.Edited != nil {
		editedTS = m.Edited.TS
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO messages(channel, ts, user, text, thread_ts, subtype, edited_ts, raw_json)
		VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(channel, ts) DO UPDATE SET
		    user=excluded.user, text=excluded.text, thread_ts=excluded.thread_ts,
		    subtype=excluded.subtype, edited_ts=excluded.edited_ts, raw_json=excluded.raw_json`,
		channel, m.TS, m.User, m.Text, m.ThreadTS, m.Subtype, editedTS, string(raw))
	return err
}

// RecentMessages returns the last n messages from a channel, oldest first.
func (s *Store) RecentMessages(ctx context.Context, channel string, n int) ([]api.Message, error) {
	if n <= 0 {
		n = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT raw_json FROM messages WHERE channel = ?
		 ORDER BY ts DESC LIMIT ?`, channel, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.Message
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var m api.Message
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	// Reverse to oldest-first for chronological rendering.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

// SearchMessages does a simple LIKE search across cached message text.
func (s *Store) SearchMessages(ctx context.Context, query string, limit int) ([]api.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT raw_json FROM messages WHERE text LIKE ? ORDER BY ts DESC LIMIT ?`,
		"%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.Message
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var m api.Message
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetCursor stores the read-cursor (highest seen ts) for a channel.
func (s *Store) SetCursor(ctx context.Context, channel, ts string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cursors(channel, last_ts) VALUES(?,?)
		 ON CONFLICT(channel) DO UPDATE SET last_ts=excluded.last_ts`, channel, ts)
	return err
}

func (s *Store) GetCursor(ctx context.Context, channel string) (string, error) {
	var ts string
	err := s.db.QueryRowContext(ctx, `SELECT last_ts FROM cursors WHERE channel = ?`, channel).Scan(&ts)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return ts, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type boolScanner struct{ b *bool }

func (s boolScanner) Scan(src any) error {
	switch v := src.(type) {
	case int64:
		*s.b = v != 0
	case bool:
		*s.b = v
	case nil:
		*s.b = false
	default:
		return fmt.Errorf("bool scan: %T", src)
	}
	return nil
}

func boolPtr(b *bool) any { return boolScanner{b} }

func channelKind(c api.Channel) string {
	switch {
	case c.IsIM:
		return "im"
	case c.IsMpim:
		return "mpim"
	case c.IsGroup:
		return "group"
	default:
		return "channel"
	}
}

func applyKind(c *api.Channel, k string) {
	switch k {
	case "im":
		c.IsIM = true
	case "mpim":
		c.IsMpim = true
	case "group":
		c.IsGroup = true
	case "channel":
		c.IsChannel = true
	}
}
