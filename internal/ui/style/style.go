package style

//
// Colours and the small formatters the views reach for
//

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

const (
	green  = lipgloss.Color("#22c55e")
	yellow = lipgloss.Color("#f59e0b")
	red    = lipgloss.Color("#ef4444")
	gray   = lipgloss.Color("#6b7280")
)

var (
	Title  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	Accent = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	Strong = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	Name   = lipgloss.NewStyle().Bold(true)
	Dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	Err    = lipgloss.NewStyle().Bold(true).Foreground(red)

	Alive   = lipgloss.NewStyle().Foreground(green)
	Dead    = lipgloss.NewStyle().Foreground(red)
	Pending = lipgloss.NewStyle().Foreground(yellow)
	Unknown = lipgloss.NewStyle().Foreground(gray)
)

// cuts a line to the terminal width: a wrapped line would push rows out of place
func Clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

func Pad(s string, w int) string {
	switch n := lipgloss.Width(s); {
	case w <= 0:
		return ""
	case n > w:
		return lipgloss.NewStyle().MaxWidth(w-1).Render(s) + "…"
	default:
		return s + strings.Repeat(" ", w-n)
	}
}

// a static usage bar for a 0..1 fraction
func Bar(fraction float64) string {
	color := green
	switch {
	case fraction >= 0.9:
		color = red
	case fraction >= 0.7:
		color = yellow
	}
	b := progress.New(progress.WithSolidFill(string(color)), progress.WithoutPercentage(), progress.WithWidth(12))
	b.EmptyColor = "8"
	return b.ViewAs(fraction)
}

func Bytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// how long ago t was; Expiry, how long until t
func Since(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func Expiry(t time.Time) string {
	d := time.Until(t)
	switch {
	case d < 0:
		return "expired " + Since(t)
	case d < time.Hour:
		return fmt.Sprintf("%dm left", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh left", int(d.Hours()))
	}
	return fmt.Sprintf("%dd left", int(d.Hours()/24))
}
