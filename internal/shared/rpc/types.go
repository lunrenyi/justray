package rpc

import (
	"encoding/json"
	"time"

	"github.com/luynrs/justray/internal/shared/domain"
)

type Req struct {
	Method string
	Args   Args
}

// ElevateMsg: the daemon is re-execing with tun caps, wait it out
const ElevateMsg = "granting permissions"

type Args struct {
	ID       string
	Sub      string
	URL      string
	Dir      int
	Tun      bool
	Settings domain.Settings
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
	Traffic   domain.Traffic
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
	Blocked   bool
}
