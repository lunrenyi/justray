package settings

import (
	"cmp"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/luynrs/justray/internal/client/tui/style"
)

func (s *Settings) View(width, height int) string {
	return strings.Join(s.lines(width, height), "\n")
}

func (s *Settings) Hints() [][2]string {
	if s.editing {
		return [][2]string{{"\U000F0311", "Apply"}, {"esc", "Cancel"}}
	}
	f, ok := s.at()
	switch {
	case ok && len(f.enum) > 0:
		return [][2]string{{"↑/↓", "Move"}, {"←/→", "Cycle"}, {"\U000F0311", "Choose"}, {"\U000F0312", "Tab"}, {"esc", "Back"}}
	case ok && f.remove != nil:
		return [][2]string{
			{"↑/↓", "Move"}, {"\U000F0312", "Tab"}, {"\U000F0311", "Edit"}, {"d", "Remove"}, {"esc", "Back"},
		}
	}
	return [][2]string{{"↑/↓", "Move"}, {"\U000F0312", "Tab"}, {"\U000F0311", "Edit"}, {"esc", "Back"}}
}

type hit struct {
	row    int
	choice string
}

func (s *Settings) lines(width, height int) []string {
	w := max(width-4, 20)
	s.input.SetWidth(max(w-8, 12))
	header := []string{style.Indent(s.tabBar(w)), ""}

	rows := s.rows()
	blocks := make([][]string, len(rows))
	picks := make([][]string, len(rows))
	for i, f := range rows {
		blocks[i], picks[i] = s.fieldBlock(f, i, w)
	}

	h := max(height-len(header), 1)
	s.scrollTo(blocks, h)

	lines := header
	hits := map[int]hit{}
	for i := s.scroll; i < len(blocks); i++ {
		if len(lines)-len(header)+len(blocks[i]) > h && i > s.scroll {
			break
		}
		for j, line := range blocks[i] {
			if len(lines)-len(header) >= h {
				break
			}
			line = style.Clip(line, width)
			hits[len(lines)] = hit{row: i, choice: picks[i][j]}
			lines = append(lines, line)
		}
	}
	s.hits = hits
	return lines
}

func (s *Settings) scrollTo(blocks [][]string, h int) {
	s.scroll = min(max(s.scroll, 0), max(len(blocks)-1, 0))
	if s.cursor < s.scroll {
		s.scroll = s.cursor
	}
	for s.scroll < s.cursor && s.span(blocks, s.scroll, s.cursor) > h {
		s.scroll++
	}
	for s.scroll > 0 && s.span(blocks, s.scroll-1, s.cursor) <= h {
		s.scroll--
	}
}

func (s *Settings) span(blocks [][]string, from, to int) int {
	total := 0
	for i := from; i <= to && i < len(blocks); i++ {
		total += len(blocks[i])
	}
	return total
}

func (s *Settings) tabBar(width int) string {
	var b strings.Builder
	for i, t := range tabs {
		b.WriteString(style.Segment(" "+t.name+" ", i == s.tab))
	}
	return style.Clip(b.String(), width)
}

func tabAt(x int) (int, bool) {
	// same segments tabBar renders
	pos := 2 // style.Indent on the header line
	for i, t := range tabs {
		w := lipgloss.Width(style.Segment(" "+t.name+" ", false))
		if x >= pos && x < pos+w {
			return i, true
		}
		pos += w
	}
	return 0, false
}

// fieldBlock renders one row, blank line above non-list rows
func (s *Settings) fieldBlock(f field, i, width int) (lines, choices []string) {
	selected := i == s.cursor

	bar := "    "
	if selected {
		bar = "  " + style.Accent.Render("┃") + " "
	}

	switch {
	case f.header:
		lines, choices = []string{bar + f.name}, []string{""}
	case f.bare:
		text := style.Dim.Render(f.name)
		if selected && s.editing {
			text = s.input.View()
		}
		lines, choices = []string{bar + text}, []string{""}
	default:
		lines = []string{bar + f.name}
		choices = []string{""}
		value, picks := s.valueLines(f, selected, bar)
		lines = append(lines, value...)
		choices = append(choices, picks...)
		if selected && s.err != "" {
			lines = append(lines, bar+style.Err.Render(style.Clip(style.FirstLine(s.err), width-4)))
			choices = append(choices, "")
		}
	}

	if i > 0 && !f.bare {
		lines = append([]string{""}, lines...)
		choices = append([]string{""}, choices...)
	}
	return lines, choices
}

func (s *Settings) valueLines(f field, selected bool, bar string) (lines, choices []string) {
	if selected && s.editing {
		return []string{bar + s.input.View()}, []string{""}
	}

	if len(f.enum) > 0 && selected {
		cur := f.get(s.cur)
		for _, opt := range f.enum {
			line := bar + style.Dim.Render("○ "+opt)
			if opt == cur {
				line = bar + style.Accent.Render("● "+opt)
			}
			lines = append(lines, line)
			choices = append(choices, opt)
		}
		return lines, choices
	}

	v := f.get(s.cur)
	if v == "" {
		v = cmp.Or(f.hint, "auto")
	}
	return []string{bar + style.Dim.Render(v)}, []string{""}
}
