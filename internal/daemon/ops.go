package daemon

import (
	"fmt"
	"maps"
	"os"

	"github.com/luynrs/justxray/internal/daemon/store"
	"github.com/luynrs/justxray/internal/daemon/xray"
	"github.com/luynrs/justxray/internal/parser/proxy"
)

const (
	socksPort = 1080 // TODO: mixed port (emm.. idk how on xray) or settings ui
	httpPort  = 1081
)

func (s *Server) connect(id string) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs, err := s.store.Subscriptions()
	if err != nil {
		return Status{}, err
	}
	n, sub, ok := find(subs, id)
	if !ok {
		return Status{}, fmt.Errorf("node %q not found", id)
	}
	if err := s.start(n, sub); err != nil {
		return Status{}, err
	}
	s.broadcast()
	return s.status(), nil
}

func (s *Server) disconnect() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := s.runner.Status().NodeName
	s.runner.Stop()
	s.sub = ""
	s.store.SetActive("")
	if name != "" {
		s.log.Printf("disconnected from %s", name)
	}
	s.broadcast()
	return s.status(), nil
}

// assumes s.mu held
func (s *Server) start(n proxy.Node, sub string) error {
	cfg, err := xray.Build(n, socksPort, httpPort)
	if err != nil {
		return err
	}
	path := xrayConf(s.dir)
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := s.runner.Start(path, n.ID, n.Name); err != nil {
		return err
	}

	s.sub = sub
	if err := s.store.SetActive(n.ID); err != nil {
		s.log.Printf("could not persist the active node: %v", err)
	}
	s.log.Printf("connected to %s (%s %s:%d, pid %d)", n.Name, n.Protocol, n.Server, n.Port, s.runner.Status().PID)
	return nil
}

// assumes s.mu held
func (s *Server) status() Status {
	p := s.runner.Status()
	st := Status{
		Connected: p.Running,
		Sub:       s.sub,
		NodeID:    p.NodeID,
		NodeName:  p.NodeName,
		PID:       p.PID,
		Uptime:    int64(p.Uptime.Seconds()),
		LastErr:   p.LastErr,
	}
	if st.Connected {
		st.Socks, st.HTTP = socksPort, httpPort
	}
	return st
}

func (s *Server) nodes() ([]Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs, err := s.store.Subscriptions()
	if err != nil {
		return nil, err
	}
	out := []Node{} // never nil: the TUI tells "no nodes" from "not loaded yet"
	for _, sub := range subs {
		for _, n := range sub.Nodes {
			item := Node{
				ID: n.ID, Name: n.Name, Protocol: string(n.Protocol),
				Server: n.Server, Port: n.Port,
				Sub: sub.ID, SubName: sub.Name,
			}
			if p, ok := s.probes[n.ID]; ok {
				item.Probed, item.Alive, item.MS = true, p.alive, p.ms
			}
			out = append(out, item)
		}
	}
	return out, nil
}

// one node if id is set, else every node in sub, else all of them
func (s *Server) probe(sub, id string) ([]Node, error) {
	s.mu.Lock()
	subs, err := s.store.Subscriptions()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}

	var targets []proxy.Node
	for _, x := range subs {
		if sub != "" && x.ID != sub {
			continue
		}
		for _, n := range x.Nodes {
			if id == "" || n.ID == id {
				targets = append(targets, n)
			}
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("nothing to probe")
	}

	results, err := s.probeNodes(targets)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	maps.Copy(s.probes, results)
	s.mu.Unlock()
	return s.nodes()
}

func find(subs []store.Subscription, nodeID string) (proxy.Node, string, bool) {
	for _, sub := range subs {
		for _, n := range sub.Nodes {
			if n.ID == nodeID {
				return n, sub.ID, true
			}
		}
	}
	return proxy.Node{}, "", false
}
