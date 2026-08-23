// Package tree lays out subscription and node rows
package tree

import (
	"strings"

	"github.com/luynrs/justray/internal/shared/rpc"
)

type Kind int

const (
	Header Kind = iota
	Node
	Gap
	Meta
)

type Row struct {
	Kind Kind
	Sub  rpc.Sub
	Node rpc.Node
}

func (r Row) SubID() string {
	if r.Kind == Node {
		return r.Node.Sub
	}
	return r.Sub.ID
}

func (r Row) Selectable() bool { return r.Kind == Header || r.Kind == Node }

type Data struct {
	Subs       []rpc.Sub
	Nodes      []rpc.Node
	Collapsed  map[string]bool
	Probing    map[string]bool
	Refreshing map[string]bool
	Query      string
	Status     rpc.Status
	Live       bool
	Spinner    string
}

func (d Data) connected() bool { return d.Live && d.Status.Connected }

func (d Data) Rows() []Row {
	q := strings.ToLower(strings.TrimSpace(d.Query))

	var rows []Row
	for _, sub := range d.Subs {
		nodes := d.SubNodes(sub.ID)
		if q != "" && !strings.Contains(strings.ToLower(sub.Name), q) {
			nodes = matching(nodes, q)
			if len(nodes) == 0 {
				continue
			}
		}
		if len(rows) > 0 {
			rows = append(rows, Row{Kind: Gap})
		}

		if sub.Direct && len(nodes) == 1 {
			rows = append(rows, Row{Kind: Node, Node: nodes[0]})
			continue
		}

		rows = append(rows, Row{Kind: Header, Sub: sub}, Row{Kind: Meta, Sub: sub})
		for _, n := range nodes {
			if q != "" || !d.Collapsed[sub.ID] || (d.connected() && d.Status.NodeID == n.ID) {
				rows = append(rows, Row{Kind: Node, Node: n})
			}
		}
	}
	return rows
}

func (d Data) SubNodes(subID string) []rpc.Node {
	var out []rpc.Node
	for _, n := range d.Nodes {
		if n.Sub == subID {
			out = append(out, n)
		}
	}
	return out
}

func (d Data) SubName(id string) string {
	for _, s := range d.Subs {
		if s.ID == id {
			return s.Name
		}
	}
	return ""
}

func matching(nodes []rpc.Node, q string) []rpc.Node {
	var out []rpc.Node
	for _, n := range nodes {
		if strings.Contains(strings.ToLower(n.Name+" "+n.Protocol+" "+n.Server), q) {
			out = append(out, n)
		}
	}
	return out
}

func Selectable(rows []Row) []int {
	var out []int
	for i, r := range rows {
		if r.Selectable() {
			out = append(out, i)
		}
	}
	return out
}

func At(rows []Row, cursor int) (Row, bool) {
	sel := Selectable(rows)
	if cursor < 0 || cursor >= len(sel) {
		return Row{}, false
	}
	return rows[sel[cursor]], true
}

func Clamp(rows []Row, cursor, scroll, height int) (int, int) {
	sel := Selectable(rows)
	if len(sel) == 0 {
		return 0, 0
	}
	cursor = min(max(cursor, 0), len(sel)-1)

	pos := sel[cursor]
	if pos < scroll {
		scroll = pos
	}
	if pos >= scroll+height {
		scroll = pos - height + 1
	}
	return cursor, min(max(scroll, 0), max(len(rows)-height, 0))
}

// Point maps a screen line to a cursor position
func Point(rows []Row, scroll, height, top, y int) (cursor int, ok bool) {
	i := scroll + y - top
	if y < top || y >= top+height || i < 0 || i >= len(rows) || !rows[i].Selectable() {
		return 0, false
	}
	return len(Selectable(rows[:i])), true
}
