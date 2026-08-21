package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func input(prompt, placeholder string, limit int) textinput.Model {
	t := textinput.New()
	t.Prompt, t.Placeholder, t.CharLimit = prompt, placeholder, limit
	return t
}

func (m Model) addKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
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
}

func (m Model) filterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
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

func (m *Model) startAdding() tea.Cmd {
	m.adding = true
	m.url.SetValue("")
	m.url.Focus()
	return textinput.Blink
}

func (m *Model) startFiltering() tea.Cmd {
	m.filtering = true
	m.filter.SetValue(m.query)
	m.filter.CursorEnd()
	m.filter.Focus()
	return textinput.Blink
}
