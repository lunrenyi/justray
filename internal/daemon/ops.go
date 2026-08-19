package daemon

import (
	"context"
	"fmt"
	"maps"
	"time"

	sbox "github.com/sagernet/sing-box"

	"github.com/luynrs/justray/internal/daemon/core"
	"github.com/luynrs/justray/internal/daemon/elevate"
	"github.com/luynrs/justray/internal/daemon/store"
	"github.com/luynrs/justray/internal/parser/proxy"
)

const (
	port         = 10808 // TODO: settings ui
	tunInterface = "justray0"
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

	name := s.node.Name
	s.clear()
	if name != "" {
		s.log.Printf("disconnected from %s", name)
	}
	s.broadcast()
	return s.status(), nil
}

// assumes s.mu held
func (s *Server) stop() {
	if s.inst != nil {
		s.inst.Close()
		s.inst, s.node = nil, proxy.Node{}
	}
}

func (s *Server) clear() {
	s.stop()
	s.sub, s.lastErr = "", ""
	if err := s.store.SetActive(""); err != nil {
		s.log.Printf("could not clear the active node: %v", err)
	}
}

func (s *Server) start(n proxy.Node, sub string) error {
	iface := ""
	if s.tun {
		iface = tunInterface
	}
	s.stop()

	opts, err := core.Build(n, port, coreLog(s.dir), iface)
	if err != nil {
		return err
	}
	inst, err := sbox.New(sbox.Options{Options: *opts, Context: core.Context(context.Background())})
	if err == nil {
		err = inst.Start()
	}
	if err != nil {
		s.lastErr = err.Error()
		if iface != "" && elevate.Needed(err) {
			go elevate.Tun(s.log, s.dir)
			return fmt.Errorf("granting tun permission, reconnecting…")
		}
		return err
	}

	s.inst, s.node, s.sub, s.started, s.lastErr = inst, n, sub, time.Now(), ""
	if err := s.store.SetActive(n.ID); err != nil {
		s.log.Printf("could not persist the active node: %v", err)
	}
	s.log.Printf("connected to %s (%s %s:%d)", n.Name, n.Protocol, n.Server, n.Port)
	return nil
}

// tun mode toggle
func (s *Server) setTun(enable bool) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tun = enable
	if s.inst == nil {
		return s.status(), nil
	}

	n, sub := s.node, s.sub
	if err := s.start(n, sub); err != nil {
		return Status{}, err
	}
	s.broadcast()
	return s.status(), nil
}

func (s *Server) status() Status {
	st := Status{Port: port, Tun: s.tun, LastErr: s.lastErr}
	if s.inst != nil {
		st.Connected, st.Sub = true, s.sub
		st.NodeID, st.NodeName = s.node.ID, s.node.Name
		st.Uptime = int64(time.Since(s.started).Seconds())
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
