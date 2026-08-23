// Package style holds the colours and line helpers
package style

import (
	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"
)

var (
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

// Segment is one tab, same width either way
func Segment(s string, active bool) string {
	if !active {
		return " " + Dim.Render(s) + " "
	}
	return pillCap.Render("▐") + pill.Render(s) + pillCap.Render("▌")
}

func Bar(fraction float64) string {
	fill := green
	switch {
	case fraction >= 0.9:
		fill = red
	case fraction >= 0.7:
		fill = yellow
	}
	b := progress.New(progress.WithColors(fill), progress.WithoutPercentage(), progress.WithWidth(12))
	b.EmptyColor = lipgloss.Color("8")
	return b.ViewAs(fraction)
}
