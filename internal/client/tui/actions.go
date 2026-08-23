package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/luynrs/justray/internal/client/tui/tree"
	"github.com/luynrs/justray/internal/shared/rpc"
)

func (m Model) activate() (tea.Model, tea.Cmd) {
	r, ok := m.at()
	if !ok {
		return m, nil
	}
	if r.Kind == tree.Header {
		m.collapsed[r.Sub.ID] = !m.collapsed[r.Sub.ID]
		m.clamp()
		return m, nil
	}
	if m.connecting {
		return m, nil
	}

	m.connecting = true
	act := m.client.Disconnect
	if !m.connected() || m.status.NodeID != r.Node.ID {
		id := r.Node.ID
		act = func() (rpc.Status, error) { return m.client.Connect(id) }
	}
	return m, tea.Batch(m.spin.Tick, connectCmd(func() error { _, err := act(); return err }))
}

func (m Model) collapse() (tea.Model, tea.Cmd) {
	r, ok := m.at()
	if !ok {
		return m, nil
	}
	m.collapsed[r.SubID()] = true
	if r.Kind == tree.Node {
		rows := m.rows()
		for i, idx := range tree.Selectable(rows) {
			if rows[idx].Kind == tree.Header && rows[idx].Sub.ID == r.SubID() {
				m.cursor = i
				break
			}
		}
	}
	m.clamp()
	return m, nil
}

func (m Model) expand() (tea.Model, tea.Cmd) {
	if r, ok := m.at(); ok {
		m.collapsed[r.SubID()] = false
		m.clamp()
	}
	return m, nil
}

func (m Model) probe() (tea.Model, tea.Cmd) {
	r, ok := m.at()
	if !ok {
		return m, nil
	}
	m.probing = map[string]bool{}
	if r.Kind == tree.Node {
		m.probing[r.Node.ID] = true
		return m, probeCmd(m.client, "", r.Node.ID)
	}
	for _, n := range m.data().SubNodes(r.Sub.ID) {
		m.probing[n.ID] = true
	}
	return m, probeCmd(m.client, r.Sub.ID, "")
}

func (m Model) probeAll() (tea.Model, tea.Cmd) {
	m.probing = map[string]bool{}
	for _, n := range m.nodes {
		m.probing[n.ID] = true
	}
	return m, probeCmd(m.client, "", "")
}

func (m Model) refresh() (tea.Model, tea.Cmd) {
	r, ok := m.at()
	if !ok {
		return m, nil
	}
	id := r.SubID()
	m.refreshing = map[string]bool{id: true}
	return m, tea.Batch(m.spin.Tick, act(m.client, func() error { _, err := m.client.Refresh(id); return err }))
}

func (m Model) refreshAll() (tea.Model, tea.Cmd) {
	m.refreshing = map[string]bool{}
	for _, sub := range m.subs {
		m.refreshing[sub.ID] = true
	}
	return m, tea.Batch(m.spin.Tick, act(m.client, func() error { _, err := m.client.RefreshAll(); return err }))
}

func (m Model) setTun(enable bool) (tea.Model, tea.Cmd) {
	if m.connecting {
		return m, nil
	}
	m.connecting = true
	return m, tea.Batch(m.spin.Tick, connectCmd(func() error { _, err := m.client.SetTun(enable); return err }))
}
