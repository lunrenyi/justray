package ui

import (
	"io"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/luynrs/justray/internal/daemon"
)

type Model struct {
	client *daemon.Client

	subs  []daemon.Sub
	nodes []daemon.Node
	nameW int

	collapsed  map[string]bool
	probing    map[string]bool
	refreshing map[string]bool
	spin       spinner.Model
	cursor     int
	scroll     int
	wheel      time.Time

	adding    bool
	url       textinput.Model
	filtering bool
	filter    textinput.Model
	query     string

	confirm bool

	status   daemon.Status
	live     bool
	since    time.Time
	statusCh chan daemon.Status

	err string

	w, h     int
	quitting bool
}

func New(c *daemon.Client) Model {
	return Model{
		client:    c,
		collapsed: map[string]bool{},
		spin:      spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		url:       input("Add:  ", "subscription URL, or a vless:// vmess:// trojan:// ss:// link", 2048),
		filter:    input("~ ", "filter by name, protocol, server…", 128),
		statusCh:  make(chan daemon.Status),
	}
}

func input(prompt, placeholder string, limit int) textinput.Model {
	t := textinput.New()
	t.Prompt, t.Placeholder, t.CharLimit = prompt, placeholder, limit
	return t
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadCmd(m.client), watch(m.client, m.statusCh), next(m.statusCh), tickCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.url.Width = max(msg.Width-12, 10)
		m.filter.Width = m.url.Width
		m.clamp()

	case tea.KeyMsg:
		return m.key(msg)

	case tea.MouseMsg:
		return m.wheelScroll(msg)

	case tick:
		return m, tickCmd()

	case spinner.TickMsg:
		if len(m.refreshing) == 0 {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case loaded:
		m.err = ""
		if msg.err != nil {
			m.err = msg.err.Error()
			m.probing, m.refreshing = nil, nil
		}
		if msg.subs != nil {
			m.subs, m.refreshing = msg.subs, nil
		}
		if msg.nodes != nil {
			m.nodes, m.probing = msg.nodes, nil
			m.nameW = nameWidth(m.nodes)
		}
		m.clamp()

	case pushed:
		st := daemon.Status(msg)
		switch {
		case !st.Connected:
			m.since = time.Time{}
		case m.since.IsZero() || st.NodeID != m.status.NodeID:
			m.since = time.Now().Add(-time.Duration(st.Uptime) * time.Second)
		}
		m.status, m.live = st, true
		return m, next(m.statusCh)
	}
	return m, nil
}

func (m Model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m.quit()
	}

	switch {
	case m.confirm:
		m.confirm = false
		if k == "y" {
			return m.remove()
		}
		return m, nil

	case m.adding:
		switch k {
		case "esc":
			m.adding = false
			m.url.Blur()
			return m, nil
		case "enter":
			url := strings.TrimSpace(m.url.Value())
			m.adding = false
			m.url.Blur()
			if url == "" {
				m.err = "a URL or a share link is required"
				return m, nil
			}
			return m, act(m.client, func() error { _, err := m.client.AddSub(url); return err })
		}
		var cmd tea.Cmd
		m.url, cmd = m.url.Update(msg)
		return m, cmd

	case m.filtering:
		switch k {
		case "esc":
			m.filtering, m.query = false, ""
			m.filter.Blur()
			m.clamp()
			return m, nil
		case "enter":
			m.filtering = false
			m.filter.Blur()
			m.clamp()
			return m, nil
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.query = m.filter.Value()
		m.clamp()
		return m, cmd
	}

	switch k {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "left", "h":
		return m.collapse()
	case "right", "l":
		return m.expand()
	case "enter":
		return m.activate()
	case "t":
		return m.probe()
	case "T":
		return m.probeAll()
	case "r":
		return m.refresh()
	case "R":
		return m.refreshAll()
	case "m":
		return m.toggleTun()
	case "a":
		m.adding = true
		m.url.SetValue("")
		m.url.Focus()
		return m, textinput.Blink
	case "/":
		m.filtering = true
		m.filter.SetValue(m.query)
		m.filter.CursorEnd()
		m.filter.Focus()
		return m, textinput.Blink
	case "d":
		_, m.confirm = m.at()
	case "q":
		return m.quit()
	case "esc":
		if m.query != "" {
			m.query = ""
			m.clamp()
		}
	}
	return m, nil
}

func (m Model) wheelScroll(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	up := msg.Button == tea.MouseButtonWheelUp
	if m.adding || m.filtering || (!up && msg.Button != tea.MouseButtonWheelDown) {
		return m, nil
	}
	if time.Since(m.wheel) < 20*time.Millisecond {
		return m, nil
	}
	m.wheel = time.Now()
	if up {
		m.move(-1)
	} else {
		m.move(1)
	}
	return m, nil
}

func (m Model) quit() (tea.Model, tea.Cmd) {
	m.quitting = true
	return m, tea.Quit
}

func Run(c *daemon.Client) error {
	log.SetOutput(io.Discard)
	_, err := tea.NewProgram(New(c), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}
