package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/luynrs/justray/internal/daemon"
	"github.com/luynrs/justray/internal/ui/style"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	body := m.tree()
	if m.adding {
		body = m.titleLine() + "\n\n" + m.url.View()
	}
	return fit(body, m.h-footerLines) + "\n" + m.footer()
}

func (m Model) titleLine() string {
	title := style.Title.Render("JustRay")
	if m.connected() {
		title = flush(title, style.Dim.Render(fmt.Sprintf("proxy :%d", m.status.Port)), m.w)
	}
	return m.clip(title)
}

func (m Model) tree() string {
	rows := m.rows()
	h := m.height()

	lines := make([]string, 0, topLines+h)
	lines = append(lines, m.titleLine(), m.filterLine())

	switch {
	case len(rows) > 0:
		cursor := -1
		if sel := selectable(rows); m.cursor < len(sel) {
			cursor = sel[m.cursor]
		}
		for i, r := range rows[m.scroll:min(m.scroll+h, len(rows))] {
			lines = append(lines, m.row(r, m.scroll+i == cursor))
		}
	case m.query != "":
		lines = append(lines, m.clip(style.Dim.Render(fmt.Sprintf("No matches for %q", m.query))))
	default:
		lines = append(lines, m.clip(style.Dim.Render("No subscriptions yet")))
	}

	for len(lines) < topLines+h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) filterLine() string {
	switch {
	case m.filtering:
		return m.clip(m.filter.View())
	case m.query != "":
		return m.clip(style.Dim.Render(fmt.Sprintf("~ %s · esc to clear", m.query)))
	}
	return ""
}

func (m Model) row(r row, selected bool) string {
	switch r.kind {
	case rowGap:
		return ""
	case rowMeta:
		return m.clip(flush("    "+usage(r.sub), m.subMeta(r.sub), m.w))
	}

	caret := "  "
	if selected {
		caret = style.Strong.Render("❯ ")
	}
	if r.kind == rowHeader {
		return m.clip(caret + m.header(r.sub, selected))
	}
	right := info(r.node)
	if l := latency(r.node); l != "" {
		right += "  " + l
	}
	return m.clip(caret + flush(m.node(r.node, selected), right, m.w-2))
}

func (m Model) header(s daemon.Sub, selected bool) string {
	arrow := "▾"
	if m.collapsed[s.ID] {
		arrow = "▸"
	}
	name := style.Name.Render(s.Name)
	if selected {
		name = style.Strong.Render(s.Name)
	}
	return arrow + " " + name
}

func (m Model) subMeta(s daemon.Sub) string {
	if m.refreshing[s.ID] {
		return style.Pending.Render("refreshing…")
	}
	age := "never updated"
	if !s.UpdatedAt.IsZero() {
		age = "updated " + style.Since(s.UpdatedAt)
	}
	plural := "s"
	if s.Nodes == 1 {
		plural = ""
	}
	return style.Dim.Render(fmt.Sprintf("%d node%s · %s", s.Nodes, plural, age))
}

const maxNameCol = 32

func nameWidth(nodes []daemon.Node) int {
	w := 0
	for _, n := range nodes {
		w = max(w, lipgloss.Width(n.Name))
	}
	return min(w, maxNameCol)
}

func (m Model) node(n daemon.Node, selected bool) string {
	name := style.Pad(n.Name, m.nameW)
	if selected {
		name = style.Accent.Render(name)
	}
	return "  " + m.dot(n) + " " + name
}

func info(n daemon.Node) string {
	return style.Dim.Render(fmt.Sprintf("%s · %s:%d", n.Protocol, n.Server, n.Port))
}

func latency(n daemon.Node) string {
	switch {
	case !n.Probed:
		return ""
	case n.Alive:
		return style.Dim.Render(fmt.Sprintf("%dms", n.MS))
	}
	return style.Dim.Render("timeout")
}

func flush(left, right string, width int) string {
	if right == "" {
		return left
	}
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 2)
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) dot(n daemon.Node) string {
	switch {
	case m.connected() && m.status.NodeID == n.ID:
		return style.Strong.Render("●")
	case m.probing[n.ID]:
		return style.Pending.Render("●")
	case !n.Probed:
		return style.Unknown.Render("●")
	case n.Alive:
		return style.Alive.Render("●")
	}
	return style.Dead.Render("●")
}

func (m Model) footer() string {
	var status string
	switch {
	case m.confirm:
		status = style.Err.Render("delete this subscription?") + " " + style.Dim.Render("cannot be undone")
	case m.connected():
		uptime := time.Duration(time.Since(m.since).Seconds()) * time.Second
		status = style.Strong.Render(fmt.Sprintf("● %s · %s", m.status.NodeName, uptime))
		if m.status.Tun {
			status += "  " + style.Dim.Render("tun")
		}
	case m.live && m.status.LastErr != "":
		status = style.Dim.Render("○ disconnected") + "  " + style.Err.Render("last error: "+m.status.LastErr)
	case m.live:
		status = style.Dim.Render("○ disconnected")
	default:
		status = style.Dim.Render("○ connecting to the daemon…")
	}
	if m.err != "" {
		status += "   " + style.Err.Render(m.err)
	}

	keys := [][2]string{
		{"↑/↓", "Move"}, {"←/→", "Fold"}, {"↵", "Toggle"}, {"t", "Ping"}, {"r", "Refresh"},
		{"m", "TUN"}, {"/", "Filter"}, {"a", "Add"}, {"d", "Delete"}, {"q", "Quit"},
	}
	switch {
	case m.confirm:
		keys = [][2]string{{"y", "Delete"}, {"any", "Cancel"}}
	case m.adding:
		keys = [][2]string{{"↵", "Add"}, {"esc", "Cancel"}}
	case m.filtering:
		keys = [][2]string{{"type", "Filter"}, {"↵", "Apply"}, {"esc", "Clear"}}
	}
	var help []string
	used := 0
	for _, k := range keys {
		hint := style.Strong.Render(k[0]) + " " + style.Dim.Render(k[1])
		if used += lipgloss.Width(hint) + 2; used-2 > m.w {
			break
		}
		help = append(help, hint)
	}

	return "\n" + m.clip(status) + "\n" + m.clip(strings.Join(help, "  "))
}

func usage(s daemon.Sub) string {
	t := s.Traffic
	used := t.UploadBytes + t.DownloadBytes

	var parts []string
	switch {
	case t.TotalBytes > 0:
		parts = append(parts, fmt.Sprintf("%s %s %s",
			style.Dim.Render(style.Bytes(used)),
			style.Bar(float64(used)/float64(t.TotalBytes)),
			style.Dim.Render(style.Bytes(t.TotalBytes))))
	case used > 0:
		parts = append(parts, style.Dim.Render(style.Bytes(used)+" used"))
	}
	if !t.ExpiresAt.IsZero() {
		parts = append(parts, style.Dim.Render(style.Expiry(t.ExpiresAt)))
	}
	return strings.Join(parts, style.Dim.Render(" · "))
}

func (m Model) clip(s string) string { return style.Clip(s, m.w) }

func fit(body string, n int) string {
	lines := strings.Split(body, "\n")
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines[:max(n, 0)], "\n")
}
