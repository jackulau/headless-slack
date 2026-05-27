package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jacklau/headless-slack/internal/api"
	"github.com/jacklau/headless-slack/internal/rtm"
	"github.com/jacklau/headless-slack/internal/store"
)

// Run launches the TUI. ctx ends the app.
func Run(ctx context.Context, client *api.Client, st *store.Store) error {
	m := newModel(client, st)
	var p *tea.Program = tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

type channelItem struct {
	c api.Channel
}

func (c channelItem) Title() string {
	switch {
	case c.c.IsIM:
		return "@dm:" + c.c.User
	case c.c.IsMpim:
		return "(group dm) " + c.c.Name
	case c.c.IsPrivate, c.c.IsGroup:
		return "🔒 " + c.c.Name
	default:
		return "# " + c.c.Name
	}
}
func (c channelItem) Description() string {
	t := strings.SplitN(c.c.Topic.Value, "\n", 2)[0]
	if len(t) > 60 {
		t = t[:60] + "…"
	}
	return t
}
func (c channelItem) FilterValue() string { return c.c.Name }

type model struct {
	client *api.Client
	store  *store.Store

	sidebar list.Model
	msgs    viewport.Model
	input   textinput.Model

	rtmClient *rtm.Client
	rtmBus    *rtm.Bus
	rtmSub    chan rtm.Event

	users map[string]string // id → display name cache

	activeChannel string
	width, height int
	status        string
}

func newModel(c *api.Client, s *store.Store) *model {
	sl := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	sl.Title = "Channels"
	sl.SetShowHelp(false)
	sl.SetShowStatusBar(false)

	ti := textinput.New()
	ti.Placeholder = "type message — Enter to send, Ctrl-C to quit"
	ti.Prompt = "> "
	ti.CharLimit = 4000

	vp := viewport.New(0, 0)

	bus := rtm.NewBus()
	return &model{
		client:    c,
		store:     s,
		sidebar:   sl,
		msgs:      vp,
		input:     ti,
		rtmBus:    bus,
		rtmClient: rtm.NewClient(c, bus),
		rtmSub:    bus.Subscribe(128),
		users:     map[string]string{},
	}
}

func (m *model) Init() tea.Cmd {
	go func() { _ = m.rtmClient.Run(context.Background()) }()
	return tea.Batch(m.loadChannels, m.input.Focus(), m.waitRTM)
}

func (m *model) loadChannels() tea.Msg {
	chans, err := fetchAll(m.client)
	if err != nil {
		return errMsg(err)
	}
	for _, c := range chans {
		_ = m.store.PutChannel(context.Background(), c)
	}
	sort.Slice(chans, func(i, j int) bool {
		if a, b := kindOrder(chans[i]), kindOrder(chans[j]); a != b {
			return a < b
		}
		return chans[i].Name < chans[j].Name
	})
	return channelsLoadedMsg(chans)
}

func (m *model) loadHistory(channel string) tea.Cmd {
	return func() tea.Msg {
		msgs, _, err := m.client.ConversationsHistory(context.Background(), channel, 50, "", "", "")
		if err != nil {
			return errMsg(err)
		}
		sort.Slice(msgs, func(i, j int) bool { return msgs[i].TS < msgs[j].TS })
		for _, mm := range msgs {
			_ = m.store.PutMessage(context.Background(), channel, mm)
		}
		return historyLoadedMsg{channel: channel, messages: msgs}
	}
}

func (m *model) waitRTM() tea.Msg {
	ev, ok := <-m.rtmSub
	if !ok {
		return nil
	}
	return rtmEventMsg(ev)
}

type channelsLoadedMsg []api.Channel
type historyLoadedMsg struct {
	channel  string
	messages []api.Message
}
type rtmEventMsg rtm.Event
type errMsg error

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		sidebarW := msg.Width / 3
		if sidebarW < 25 {
			sidebarW = 25
		}
		mainW := msg.Width - sidebarW - 2
		m.sidebar.SetSize(sidebarW, msg.Height-1)
		m.msgs.Width = mainW
		m.msgs.Height = msg.Height - 4
		m.input.Width = mainW - 2

	case channelsLoadedMsg:
		items := make([]list.Item, 0, len(msg))
		for _, c := range msg {
			items = append(items, channelItem{c: c})
		}
		m.sidebar.SetItems(items)
		m.status = fmt.Sprintf("loaded %d channels", len(msg))

	case historyLoadedMsg:
		m.activeChannel = msg.channel
		m.msgs.SetContent(m.renderMessages(msg.messages))
		m.msgs.GotoBottom()

	case rtmEventMsg:
		if msg.Type == rtm.EventMessage && msg.Channel == m.activeChannel {
			m.appendLive(api.Message{TS: msg.TS, User: msg.User, Text: msg.Text})
		}
		cmds = append(cmds, m.waitRTM)

	case errMsg:
		m.status = "error: " + msg.Error()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.input.Focused() && m.activeChannel != "" {
				text := strings.TrimSpace(m.input.Value())
				if text != "" {
					m.input.SetValue("")
					ch := m.activeChannel
					cmds = append(cmds, func() tea.Msg {
						_, _, _ = m.client.ChatPostMessage(context.Background(), ch, text)
						return nil
					})
				}
				goto fall
			}
			if it, ok := m.sidebar.SelectedItem().(channelItem); ok {
				cmds = append(cmds, m.loadHistory(it.c.ID))
			}
		case "tab":
			if m.input.Focused() {
				m.input.Blur()
			} else {
				cmds = append(cmds, m.input.Focus())
			}
		}
	fall:
	}

	if !m.input.Focused() {
		var c tea.Cmd
		m.sidebar, c = m.sidebar.Update(msg)
		cmds = append(cmds, c)
	} else {
		var c tea.Cmd
		m.input, c = m.input.Update(msg)
		cmds = append(cmds, c)
	}
	var c tea.Cmd
	m.msgs, c = m.msgs.Update(msg)
	cmds = append(cmds, c)

	return m, tea.Batch(cmds...)
}

func (m *model) renderMessages(msgs []api.Message) string {
	var b strings.Builder
	for _, mm := range msgs {
		name := m.userName(mm.User)
		b.WriteString(fmt.Sprintf("%s  %s  %s\n", tsTime(mm.TS), padR(name, 16), mm.Text))
	}
	return b.String()
}

func (m *model) appendLive(mm api.Message) {
	name := m.userName(mm.User)
	cur := m.msgs.View()
	_ = cur
	line := fmt.Sprintf("%s  %s  %s\n", tsTime(mm.TS), padR(name, 16), mm.Text)
	m.msgs.SetContent(line + "...append...\n")
}

func (m *model) userName(id string) string {
	if id == "" {
		return "(bot)"
	}
	if n, ok := m.users[id]; ok {
		return n
	}
	if u, err := m.store.GetUser(context.Background(), id); err == nil && u.ID != "" {
		n := u.Profile.DisplayName
		if n == "" {
			n = u.RealName
		}
		if n == "" {
			n = u.Name
		}
		if n == "" {
			n = id
		}
		m.users[id] = n
		return n
	}
	return id
}

func tsTime(ts string) string {
	dot := strings.IndexByte(ts, '.')
	sec := ts
	if dot >= 0 {
		sec = ts[:dot]
	}
	var n int64
	_, _ = fmt.Sscanf(sec, "%d", &n)
	if n == 0 {
		return ts
	}
	return time.Unix(n, 0).Format("15:04")
}

func padR(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	statusBar  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func (m *model) View() string {
	left := m.sidebar.View()
	hdr := titleStyle.Render(m.activeChannelHeader())
	right := lipgloss.JoinVertical(lipgloss.Left, hdr, m.msgs.View(), m.input.View())
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	return lipgloss.JoinVertical(lipgloss.Left, body, statusBar.Render(m.status))
}

func (m *model) activeChannelHeader() string {
	if m.activeChannel == "" {
		return "slk — select a channel (↑/↓ Enter to open, Tab to type)"
	}
	return "slk — " + m.activeChannel
}

func fetchAll(c *api.Client) ([]api.Channel, error) {
	var all []api.Channel
	cursor := ""
	for {
		p, next, err := c.ConversationsList(context.Background(), "", cursor, 200)
		if err != nil {
			return nil, err
		}
		all = append(all, p...)
		if next == "" {
			return all, nil
		}
		cursor = next
	}
}

func kindOrder(c api.Channel) int {
	switch {
	case c.IsChannel && !c.IsPrivate:
		return 0
	case c.IsPrivate, c.IsGroup:
		return 1
	case c.IsMpim:
		return 2
	case c.IsIM:
		return 3
	default:
		return 4
	}
}
