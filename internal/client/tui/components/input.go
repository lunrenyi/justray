// Package components holds the reusable view pieces
package components

import "charm.land/bubbles/v2/textinput"

func Input(prompt, placeholder string, limit int) textinput.Model {
	t := textinput.New()
	t.Prompt, t.Placeholder, t.CharLimit = prompt, placeholder, limit
	return t
}
