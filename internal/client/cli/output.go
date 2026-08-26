package cli

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"

	"github.com/luynrs/justray/internal/client/tui/style"
	"github.com/luynrs/justray/internal/shared/rpc"
)

func out(s string) { _, _ = lipgloss.Println(s) }

func done(text string) { out(style.Alive.Bold(true).Render("✓") + " " + text) }

func label(text string) string { return style.Dim.Render(text + ":") }

func fields(pairs ...[2]string) { out(fieldLines(pairs...)) }

func fieldLines(pairs ...[2]string) string {
	w := 0
	for _, p := range pairs {
		w = max(w, lipgloss.Width(p[0]))
	}
	lines := make([]string, len(pairs))
	for i, p := range pairs {
		lines[i] = "  " + style.Pad(label(p[0]), w+1) + " " + p[1]
	}
	return strings.Join(lines, "\n")
}

func state(st rpc.Status) string {
	switch {
	case st.Connected:
		return "connected via " + strings.ToUpper(modeWord(st.Tun))
	case st.Blocked:
		return "blocked (kill switch)"
	}
	return "disconnected"
}

func stateHeadline(st rpc.Status) {
	text := state(st)
	if st.Connected {
		done(upperFirst(text))
		return
	}
	color := style.Dim
	if st.Blocked {
		color = style.Pending
	}
	out(color.Render("·") + " " + upperFirst(text))
}

func (a *app) statusFields(st rpc.Status, n rpc.Node) [][2]string {
	f := [][2]string{{"Node", a.nodeName(st.NodeName, st.NodeID)}}
	f = append(f, nodeFields(n)...)
	if st.Uptime > 0 {
		f = append(f, [2]string{"Uptime", style.Uptime(time.Duration(st.Uptime) * time.Second)})
	}
	return f
}

func (a *app) nodeDetails(st rpc.Status) {
	n, _ := a.resolveNode(st.NodeID)
	fields(a.statusFields(st, n)...)
}

func nodeFields(n rpc.Node) [][2]string {
	if n.Server == "" {
		return nil
	}
	return [][2]string{
		{"Server", fmt.Sprintf("%s:%d", n.Server, n.Port)},
		{"Protocol", n.Protocol},
	}
}

func (a *app) nodeName(name, id string) string {
	if id == "" {
		return style.Sanitize(name, a.emoji)
	}
	return style.Sanitize(name, a.emoji) + " " + style.Dim.Render("("+id+")")
}

func modeWord(tun bool) string {
	if tun {
		return "tun"
	}
	return "proxy"
}

func upperFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// Fail prints what the CLI died on
func Fail(err error) {
	_, _ = lipgloss.Fprintln(os.Stderr, style.Err.Bold(true).Render("✗"), upperFirst(err.Error()))
}

func warn(msg string) {
	if msg != "" {
		out("  " + style.Err.Render(style.FirstLine(msg)))
	}
}

func spin(text string) func() {
	if fi, err := os.Stdout.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return func() {}
	}
	s := spinner.MiniDot
	quit, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		for i := 0; ; i++ {
			_, _ = lipgloss.Printf("\r%s %s", style.Accent.Render(s.Frames[i%len(s.Frames)]), text)
			select {
			case <-quit:
				return
			case <-time.After(s.FPS):
			}
		}
	}()
	return func() {
		close(quit)
		<-stopped
		_, _ = lipgloss.Print("\r\x1b[K")
	}
}
