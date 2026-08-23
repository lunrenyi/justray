package style

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

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

// Flush right-aligns right, at least two spaces apart
func Flush(left, right string, width int) string {
	if right == "" {
		return left
	}
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 2)
	return left + strings.Repeat(" ", gap) + right
}

func Fit(body string, n int) string {
	lines := strings.Split(body, "\n")
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines[:max(n, 0)], "\n")
}

func Indent(s string) string {
	var b strings.Builder
	for line := range strings.Lines(s) {
		b.WriteString("  " + line)
	}
	return b.String()
}

func FirstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// Sanitize drops control characters from node names
func Sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
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
