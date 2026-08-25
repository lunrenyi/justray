// Package engine is the engine interface, implemented at engine/singbox
package engine

import "github.com/luynrs/justray/internal/shared/domain"

// Engine drives one running proxy/TUN session for a single node.
type Engine interface {
	// Start builds and starts a fresh instance for n, tun or not
	Start(n domain.Node, tun bool) error
	// Stage builds the kill-switch instance without starting its tun
	Stage() error
	// Block arms the kill switch, a rejecting tun
	Block() error
	// Swap hot-swaps the active outbound/endpoint to n without rebuilding.
	Swap(n domain.Node) error
	// TunAdd/TunRemove hot-toggle the TUN inbound on an already-running engine.
	TunAdd() error
	TunRemove() error
	// Close tears the instance down, waiting for the tun to disappear
	Close() error
}

// Result is one node's probed reachability.
type Result struct {
	Alive bool
	MS    int
}

type New func(s domain.Settings, logPath string) Engine

type Probe func(nodes []domain.Node, s domain.Settings, logPath string) (map[string]Result, error)
