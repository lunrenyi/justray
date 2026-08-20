package daemon

import (
	"encoding/json"
	"time"

	"github.com/luynrs/justray/internal/daemon/store"
)

type Req struct {
	Method string
	Args   Args
}

type Args struct {
	ID  string // subscription id, or node id, depending on the method
	Sub string
	URL string
	Tun bool
}

type Resp struct {
	OK     bool
	Result json.RawMessage
	Error  string
}

type Sub struct {
	ID        string
	Name      string
	Nodes     int
	UpdatedAt time.Time
	Traffic   store.Traffic
	Direct    bool // a bare share link
}

type Node struct {
	ID       string
	Name     string
	Protocol string
	Server   string
	Port     int
	Sub      string

	// false until Probe has run
	Probed bool
	Alive  bool
	MS     int
}

type Status struct {
	Connected bool
	NodeID    string
	NodeName  string
	Uptime    int64 // seconds
	LastErr   string
	Port      int
	Tun       bool
}
