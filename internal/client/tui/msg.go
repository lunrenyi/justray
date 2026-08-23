package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/luynrs/justray/internal/shared/rpc"
)

type loaded struct {
	subs  []rpc.Sub
	nodes []rpc.Node
	err   error
}

type pushed struct {
	st   rpc.Status
	live bool
}

type connectResult struct{ err error }

func connectCmd(fn func() error) tea.Cmd {
	return func() tea.Msg { return connectResult{fn()} }
}

type tick struct{}

func load(c *rpc.Client) tea.Msg {
	subs, err := c.Subs()
	if err != nil {
		return loaded{err: err}
	}
	nodes, err := c.Nodes()
	return loaded{subs: subs, nodes: nodes, err: err}
}

func loadCmd(c *rpc.Client) tea.Cmd {
	return func() tea.Msg { return load(c) }
}

func act(c *rpc.Client, fn func() error) tea.Cmd {
	return func() tea.Msg {
		if err := fn(); err != nil {
			return loaded{err: err}
		}
		return load(c)
	}
}

func probeCmd(c *rpc.Client, sub, id string) tea.Cmd {
	return func() tea.Msg {
		nodes, err := c.Probe(sub, id)
		return loaded{nodes: nodes, err: err}
	}
}

func watch(c *rpc.Client, ch chan<- pushed) tea.Cmd {
	return func() tea.Msg {
		go func() {
			backoff := time.Second
			for {
				c.Watch(func(st rpc.Status) {
					ch <- pushed{st: st, live: true}
					backoff = time.Second
				})
				ch <- pushed{}
				time.Sleep(backoff)
				if backoff < 10*time.Second {
					backoff *= 2
				}
			}
		}()
		return nil
	}
}

func next(ch <-chan pushed) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tick{} })
}
