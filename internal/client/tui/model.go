package tui

import (
	"io"
	"log"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/luynrs/justray/internal/shared/rpc"
)

type Model struct {
	client *rpc.Client

	subs  []rpc.Sub
	nodes []rpc.Node

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

	confirmSub string

	status     rpc.Status
	live       bool
	since      time.Time
	statusCh   chan rpc.Status
	connecting bool

	err string

	w, h     int
	quitting bool
}

func New(c *rpc.Client) Model {
	return Model{
		client:    c,
		collapsed: map[string]bool{},
		spin:      spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		url:       input("Add:  ", "subscription URL, or a vless://, vmess://, trojan://, ss://, etc. link", 2048),
		filter:    input("", "type to filter...", 128),
		statusCh:  make(chan rpc.Status),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadCmd(m.client), watch(m.client, m.statusCh), next(m.statusCh), tickCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.url.Width = max(msg.Width-12, 10)
		m.clamp()

	case tea.KeyMsg:
		return m.key(msg)

	case tea.MouseMsg:
		return m.mouse(msg)

	case tick:
		return m, tickCmd()

	case spinner.TickMsg:
		if len(m.refreshing) == 0 && !m.connecting {
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
		}
		m.clamp()

	case connectResult:
		m.connecting = false
		m.err = ""
		if msg.err != nil {
			m.err = msg.err.Error()
		}

	case pushed:
		st := rpc.Status(msg)
		m.since = time.Now().Add(-time.Duration(st.Uptime) * time.Second)
		m.status, m.live, m.err = st, true, ""
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
	case m.confirmSub != "":
		id := m.confirmSub
		m.confirmSub = ""
		if k == "y" {
			return m.remove(id)
		}
		return m, nil

	case m.adding:
		return m.addKey(msg)

	case m.filtering:
		return m.filterKey(msg)
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
		return m.setTun(!m.status.Tun)
	case "a":
		return m, m.startAdding()
	case "/":
		return m, m.startFiltering()
	case "d":
		if r, ok := m.at(); ok {
			m.confirmSub = r.subID()
		}
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

func (m Model) click(x, y int) (tea.Model, tea.Cmd) {
	if y == 0 {
		if tun, ok := modeAt(x, m.w); ok {
			return m.setTun(tun)
		}
		return m, nil
	}
	focused, ok := m.point(y)
	if r, _ := m.at(); ok && (focused || r.kind == rowHeader) {
		return m.activate()
	}
	return m, nil
}

func (m Model) mouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.adding || m.confirmSub != "" {
		return m, nil
	}
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		return m.click(msg.X, msg.Y)
	}

	up := msg.Button == tea.MouseButtonWheelUp
	if m.filtering || (!up && msg.Button != tea.MouseButtonWheelDown) {
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

func Run(c *rpc.Client) error {
	log.SetOutput(io.Discard)
	_, err := tea.NewProgram(New(c), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}
