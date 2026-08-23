// Package subscriptions renders subscription blocks and the add prompt
package subscriptions

import (
	"fmt"
	"strings"

	"github.com/luynrs/justray/internal/client/tui/style"
	"github.com/luynrs/justray/internal/shared/rpc"
)

func Header(s rpc.Sub, collapsed, selected bool) string {
	arrow := "▾"
	if collapsed {
		arrow = "▸"
	}
	clean := style.Sanitize(s.Name)
	name := style.Name.Render(clean)
	if selected {
		name = style.Strong.Render(clean)
	}
	return arrow + " " + name
}

func Meta(s rpc.Sub, refreshing bool, spinner string) string {
	age := "never updated"
	switch {
	case refreshing:
		age = "updated " + spinner + " ago"
	case !s.UpdatedAt.IsZero():
		age = "updated " + style.Since(s.UpdatedAt)
	}
	plural := "s"
	if s.Nodes == 1 {
		plural = ""
	}
	return style.Dim.Render(fmt.Sprintf("%d node%s · %s", s.Nodes, plural, age))
}

// Usage is empty when the provider reports no numbers
func Usage(s rpc.Sub) string {
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
