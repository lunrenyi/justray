package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/luynrs/justray/internal/daemon"
)

const (
	topLines    = 2 // title + filter
	footerLines = 3 // status + help
)

type rowKind int

const (
	rowHeader rowKind = iota
	rowNode
	rowGap
	rowMeta
)

type row struct {
	kind rowKind
	sub  daemon.Sub
	node daemon.Node
}

func (r row) subID() string {
	if r.kind == rowNode {
		return r.node.Sub
	}
	return r.sub.ID
}

func (r row) selectable() bool { return r.kind == rowHeader || r.kind == rowNode }

func (m Model) rows() []row {
	q := strings.ToLower(strings.TrimSpace(m.query))

	var rows []row
	for _, sub := range m.subs {
		nodes := m.subNodes(sub.ID)
		if q != "" && !strings.Contains(strings.ToLower(sub.Name), q) {
			nodes = matching(nodes, q)
			if len(nodes) == 0 {
				continue
			}
		}
		if len(rows) > 0 {
			rows = append(rows, row{kind: rowGap})
		}

		if sub.Direct && len(nodes) == 1 {
			rows = append(rows, row{kind: rowNode, node: nodes[0]})
			continue
		}

		rows = append(rows, row{kind: rowHeader, sub: sub}, row{kind: rowMeta, sub: sub})
		for _, n := range nodes {
			if q != "" || !m.collapsed[sub.ID] || (m.connected() && m.status.NodeID == n.ID) {
				rows = append(rows, row{kind: rowNode, node: n})
			}
		}
	}
	return rows
}

func (m Model) subNodes(subID string) []daemon.Node {
	var out []daemon.Node
	for _, n := range m.nodes {
		if n.Sub == subID {
			out = append(out, n)
		}
	}
	return out
}

func matching(nodes []daemon.Node, q string) []daemon.Node {
	var out []daemon.Node
	for _, n := range nodes {
		if strings.Contains(strings.ToLower(n.Name+" "+n.Protocol+" "+n.Server), q) {
			out = append(out, n)
		}
	}
	return out
}

func (m Model) connected() bool { return m.live && m.status.Connected }

func selectable(rows []row) []int {
	var out []int
	for i, r := range rows {
		if r.selectable() {
			out = append(out, i)
		}
	}
	return out
}

func (m Model) at() (row, bool) {
	rows := m.rows()
	sel := selectable(rows)
	if m.cursor < 0 || m.cursor >= len(sel) {
		return row{}, false
	}
	return rows[sel[m.cursor]], true
}

func (m Model) height() int { return max(m.h-topLines-footerLines, 1) }

func (m *Model) move(delta int) {
	m.cursor += delta
	m.clamp()
}

// scroll
func (m *Model) clamp() {
	rows := m.rows()
	sel := selectable(rows)
	if len(sel) == 0 {
		m.cursor, m.scroll = 0, 0
		return
	}
	m.cursor = min(max(m.cursor, 0), len(sel)-1)

	h := m.height()
	pos := sel[m.cursor]
	if pos < m.scroll {
		m.scroll = pos
	}
	if pos >= m.scroll+h {
		m.scroll = pos - h + 1
	}
	m.scroll = min(max(m.scroll, 0), max(len(rows)-h, 0))
}

func (m Model) activate() (tea.Model, tea.Cmd) {
	r, ok := m.at()
	if !ok {
		return m, nil
	}
	if r.kind == rowHeader {
		m.collapsed[r.sub.ID] = !m.collapsed[r.sub.ID]
		m.clamp()
		return m, nil
	}
	if m.connected() && m.status.NodeID == r.node.ID {
		return m, run(func() error { _, err := m.client.Disconnect(); return err })
	}
	return m, run(func() error { _, err := m.client.Connect(r.node.ID); return err })
}

func (m Model) collapse() (tea.Model, tea.Cmd) {
	r, ok := m.at()
	if !ok {
		return m, nil
	}
	m.collapsed[r.subID()] = true
	if r.kind == rowNode {
		rows := m.rows()
		for i, idx := range selectable(rows) {
			if rows[idx].kind == rowHeader && rows[idx].sub.ID == r.subID() {
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
		m.collapsed[r.subID()] = false
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
	if r.kind == rowNode {
		m.probing[r.node.ID] = true
		return m, probeCmd(m.client, "", r.node.ID)
	}
	for _, n := range m.subNodes(r.sub.ID) {
		m.probing[n.ID] = true
	}
	return m, probeCmd(m.client, r.sub.ID, "")
}

func (m Model) refresh() (tea.Model, tea.Cmd) {
	r, ok := m.at()
	if !ok {
		return m, nil
	}
	id := r.subID()
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

func (m Model) probeAll() (tea.Model, tea.Cmd) {
	m.probing = map[string]bool{}
	for _, n := range m.nodes {
		m.probing[n.ID] = true
	}
	return m, probeCmd(m.client, "", "")
}

func (m Model) toggleTun() (tea.Model, tea.Cmd) {
	enable := !m.status.Tun
	return m, run(func() error { _, err := m.client.SetTun(enable); return err })
}

func (m Model) remove() (tea.Model, tea.Cmd) {
	r, ok := m.at()
	if !ok {
		return m, nil
	}
	id := r.subID()
	return m, act(m.client, func() error { return m.client.RemoveSub(id) })
}
