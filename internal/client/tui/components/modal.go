package components

import "github.com/luynrs/justray/internal/client/tui/style"

type Confirm struct {
	question string
	subject  string
}

func (c *Confirm) Ask(question, subject string) {
	c.question, c.subject = question, subject
}

func (c *Confirm) Active() bool { return c.subject != "" }

func (c *Confirm) Answer(key string) (subject string, yes bool) {
	subject = c.subject
	c.question, c.subject = "", ""
	return subject, key == "y"
}

func (c *Confirm) View() string {
	if !c.Active() {
		return ""
	}
	return style.Err.Render(style.Sanitize(c.question, true))
}
