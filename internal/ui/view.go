package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/luynrs/justxray/internal/daemon"
	"github.com/luynrs/justxray/internal/ui/style"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	body := m.tree()
	if m.adding {
		body = m.line(style.Title.Render("JustXray")) + "\n\n" + m.url.View()
	}
	return pad(body, m.h-footerLines) + "\n" + m.footer()
}

func (m Model) tree() string {
	rows := m.rows()
	h := m.height()

	lines := make([]string, 0, topLines+h)
	lines = append(lines, m.line(style.Title.Render("JustXray")), m.filterLine())

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
		lines = append(lines, m.line(style.Dim.Render(fmt.Sprintf("No matches for %q", m.query))))
	default:
		lines = append(lines, m.line(style.Dim.Render("No subscriptions yet")))
	}

	for len(lines) < topLines+h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) filterLine() string {
	switch {
	case m.filtering:
		return m.line(m.filter.View())
	case m.query != "":
		return m.line(style.Dim.Render(fmt.Sprintf("~ %s · esc to clear", m.query)))
	}
	return ""
}

func (m Model) row(r row, selected bool) string {
	switch r.kind {
	case rowGap:
		return ""
	case rowMeta:
		return m.line("    " + usage(r.sub))
	}

	cursor := "  "
	if selected {
		cursor = style.Strong.Render("❯ ")
	}
	if r.kind == rowHeader {
		return m.line(cursor + m.header(r.sub, selected))
	}
	return m.line(cursor + m.node(r.node, selected))
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

	age := "never updated"
	if !s.UpdatedAt.IsZero() {
		age = "updated " + style.Since(s.UpdatedAt)
	}
	plural := "s"
	if s.Nodes == 1 {
		plural = ""
	}
	return arrow + " " + name + "  " + style.Dim.Render(fmt.Sprintf("%d node%s · %s", s.Nodes, plural, age))
}

func (m Model) node(n daemon.Node, selected bool) string {
	name := n.Name
	if selected {
		name = style.Accent.Render(name)
	}

	latency := ""
	switch {
	case !n.Probed:
	case n.Alive:
		latency = fmt.Sprintf("  %dms", n.MS)
	default:
		latency = "  timeout"
	}
	meta := style.Dim.Render(fmt.Sprintf("%s · %s:%d%s", n.Protocol, n.Server, n.Port, latency))

	return "  " + m.dot(n) + " " + name + "  " + meta
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
	case m.connected():
		uptime := time.Duration(time.Since(m.since).Seconds()) * time.Second
		status = style.Strong.Render(fmt.Sprintf("● %s · %s", m.status.NodeName, uptime)) +
			"  " + style.Dim.Render(fmt.Sprintf("socks :%d · http :%d", m.status.Socks, m.status.HTTP))
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
		{"↑/↓", "Move"}, {"↵", "Toggle"}, {"t", "Latency"}, {"~", "Filter"},
		{"a", "Add"}, {"d", "Delete"}, {"q", "Quit"},
	}
	switch {
	case m.adding:
		keys = [][2]string{{"↵", "Add"}, {"esc", "Cancel"}}
	case m.filtering:
		keys = [][2]string{{"type", "Filter"}, {"↵", "Apply"}, {"esc", "Clear"}}
	}
	help := make([]string, len(keys))
	for i, k := range keys {
		help[i] = style.Strong.Render(k[0]) + " " + style.Dim.Render(k[1])
	}

	return "\n" + m.line(status) + "\n" + m.line(strings.Join(help, "    "))
}

func usage(s daemon.Sub) string {
	var parts []string
	if t := s.Traffic; t.TotalBytes > 0 {
		used := t.UploadBytes + t.DownloadBytes
		parts = append(parts, fmt.Sprintf("%s %s %s",
			style.Dim.Render(style.Bytes(used)),
			style.Bar(float64(used)/float64(t.TotalBytes)),
			style.Dim.Render(style.Bytes(t.TotalBytes))))
	}
	if !s.Traffic.ExpiresAt.IsZero() {
		parts = append(parts, style.Dim.Render(style.Expiry(s.Traffic.ExpiresAt)))
	}
	return strings.Join(parts, style.Dim.Render(" · "))
}

func (m Model) line(s string) string { return style.Clip(s, m.w) }

func pad(body string, n int) string {
	lines := strings.Split(body, "\n")
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines[:max(n, 0)], "\n")
}
