package subscriptions

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/luynrs/justray/internal/client/tui/components"
)

type Editor struct {
	input  textinput.Model
	active bool
}

func NewEditor() *Editor {
	return &Editor{
		input: components.Input("Add:  ", "subscription URL, or a vless://, vmess://, trojan://, ss://, etc. link", 2048),
	}
}

func (e *Editor) Active() bool { return e.active }

func (e *Editor) SetWidth(w int) { e.input.SetWidth(max(w-12, 10)) }

func (e *Editor) Start() tea.Cmd {
	e.active = true
	e.input.SetValue("")
	return tea.Batch(e.input.Focus(), textinput.Blink)
}

// Key returns done on submit and cancel, url empty on cancel
func (e *Editor) Key(msg tea.KeyPressMsg) (url string, done bool, cmd tea.Cmd) {
	switch msg.String() {
	case "esc":
		e.active = false
		e.input.Blur()
		return "", true, nil
	case "enter":
		e.active = false
		e.input.Blur()
		return strings.TrimSpace(e.input.Value()), true, nil
	}
	e.input, cmd = e.input.Update(msg)
	return "", false, cmd
}

func (e *Editor) View() string { return e.input.View() }

func (e *Editor) Hints() [][2]string {
	return [][2]string{{"↵", "Add"}, {"esc", "Cancel"}}
}
