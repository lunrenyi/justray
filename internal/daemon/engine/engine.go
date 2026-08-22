// Package engine is daemon's one door to a VPN engine. sing-box types stop
// at engine/singbox — nothing above this interface knows about *sbox.Box,
// sing-box options, or its registry.
package engine

import "github.com/luynrs/justray/internal/shared/domain"

// Engine drives one running proxy/TUN session for a single node.
type Engine interface {
	// Start builds and starts a fresh instance for n. iface is the TUN
	// interface name to bring up, or "" for proxy-only mode.
	Start(n domain.Node, iface string) error
	// Swap hot-swaps the active outbound/endpoint to n without rebuilding.
	Swap(n domain.Node) error
	// TunAdd/TunRemove hot-toggle the TUN inbound on an already-running engine.
	TunAdd(iface string) error
	TunRemove(iface string) error
	// Close tears the instance down, waiting for the TUN interface (if any)
	// to actually disappear.
	Close() error
}

// Result is one node's probed reachability.
type Result struct {
	Alive bool
	MS    int
}
