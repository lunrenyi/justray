package style

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
	Err    = lipgloss.NewStyle().Foreground(red)

	pill    = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4"))
	pillCap = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))

	Alive   = lipgloss.NewStyle().Foreground(green)
	Dead    = lipgloss.NewStyle().Foreground(red)
	Pending = lipgloss.NewStyle().Foreground(yellow)
	Unknown = lipgloss.NewStyle().Foreground(gray)
)

func Segment(s string, active bool) string {
	if !active {
		return " " + Dim.Render(s) + " "
	}
	return pillCap.Render("▐") + pill.Render(s) + pillCap.Render("▌")
}

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
		t := lipgloss.NewStyle().MaxWidth(w-1).Render(s) + "…"
		if shortfall := w - lipgloss.Width(t); shortfall > 0 {
			t += strings.Repeat(" ", shortfall)
		}
		return t
	default:
		return s + strings.Repeat(" ", w-n)
	}
}

func Sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

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

func Since(t time.Time) string { return span(time.Since(t)) + " ago" }

func Expiry(t time.Time) string {
	d := time.Until(t)
	if d < 0 {
		return "expired " + Since(t)
	}
	return span(d) + " left"
}

func span(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Hour-30*time.Second:
		return fmt.Sprintf("%dm", (d+30*time.Second)/time.Minute)
	case d < 24*time.Hour-30*time.Minute:
		return fmt.Sprintf("%dh", (d+30*time.Minute)/time.Hour)
	}
	return fmt.Sprintf("%dd", (d+12*time.Hour)/(24*time.Hour))
}
