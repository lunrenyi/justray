package daemon

import (
	"encoding/json"
	"time"

	"github.com/luynrs/justxray/internal/daemon/store"
)

type Req struct {
	Method string
	Args   Args
}

type Args struct {
	ID  string // subscription id, or node id, depending on the method
	Sub string
	URL string
}

type Resp struct {
	OK     bool
	Result json.RawMessage
	Error  string
}

type Sub struct {
	ID        string
	Name      string
	URL       string
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
	SubName  string

	// false until Probe has run
	Probed bool
	Alive  bool
	MS     int
}

type Status struct {
	Connected bool
	Sub       string
	NodeID    string
	NodeName  string
	PID       int
	Uptime    int64 // seconds
	LastErr   string
	Socks     int
	HTTP      int
}
