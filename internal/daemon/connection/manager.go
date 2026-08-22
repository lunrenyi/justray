package connection

import (
	"fmt"
	"slices"
	"time"

	"github.com/luynrs/justray/internal/daemon/platform/elevate"
	"github.com/luynrs/justray/internal/daemon/store"
	"github.com/luynrs/justray/internal/shared/domain"
	"github.com/luynrs/justray/internal/shared/rpc"
)

// Restore reconnects to whatever node/mode was persisted from the last run.
func (s *Service) Restore() {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	id, err := s.store.Active()
	if err != nil || id == "" {
		return
	}
	subs, err := s.store.Subscriptions()
	if err != nil {
		s.log.Printf("restore: %v", err)
		return
	}
	n, sub, ok := find(subs, id)
	if !ok {
		s.log.Printf("restore: node %q is gone", id)
		return
	}
	if err := s.start(n, sub); err != nil {
		s.log.Printf("restore: %s: %v", n.Name, err)
		return
	}
	s.broadcast()
}

// ForgetIfRemoved clears the persisted active node and disconnects if it
// belonged to a subscription that just got deleted. subscription.Service
// can't do this itself: both the live connection and the persisted active
// id are connection's private state.
func (s *Service) ForgetIfRemoved(subID string, nodes []domain.Node) {
	if active, err := s.store.Active(); err == nil && slices.ContainsFunc(nodes, func(n domain.Node) bool { return n.ID == active }) {
		if err := s.store.SetActive(""); err != nil {
			s.log.Printf("could not clear the active node: %v", err)
		}
	}

	s.mu.Lock()
	live := s.sub == subID
	s.mu.Unlock()
	if live {
		s.Disconnect()
	}
}

func (s *Service) start(n domain.Node, sub string) error {
	s.mu.Lock()
	iface := ""
	if s.tun {
		iface = TunIface
	}
	live := s.eng
	hot := live != nil && s.tunLive == (iface != "")
	s.mu.Unlock()

	if hot {
		err := live.Swap(n)
		s.mu.Lock()
		if err != nil {
			s.lastErr = err.Error()
			s.mu.Unlock()
			return err
		}
		s.node, s.sub, s.started, s.lastErr = n, sub, time.Now(), ""
		s.mu.Unlock()
		s.persistActive(n.ID)
		s.log.Printf("connected to %s (%s %s:%d)", n.Name, n.Protocol, n.Server, n.Port)
		return nil
	}

	s.stop()

	if err := rpc.ClearLog(rpc.CoreLog(s.dir)); err != nil {
		s.log.Printf("could not truncate core log: %v", err)
	}

	eng := s.newEngine(Port, rpc.CoreLog(s.dir))
	err := eng.Start(n, iface)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.lastErr = err.Error()
		eng.Close()
		if iface != "" && elevate.Needed(err) {
			s.persistActive(n.ID)
			go elevate.Tun(s.log, s.dir)
			return fmt.Errorf("granting tun permission, reconnecting…")
		}
		return err
	}

	s.eng, s.node, s.sub, s.started, s.lastErr, s.tunLive = eng, n, sub, time.Now(), "", iface != ""
	s.persistActive(n.ID)
	s.log.Printf("connected to %s (%s %s:%d)", n.Name, n.Protocol, n.Server, n.Port)
	return nil
}

func (s *Service) stop() {
	s.mu.Lock()
	eng := s.eng
	s.eng, s.node, s.sub, s.tunLive = nil, domain.Node{}, "", false
	s.mu.Unlock()

	if eng == nil {
		return
	}
	if err := eng.Close(); err != nil {
		s.log.Printf("closing the engine: %v", err)
	}
}

func (s *Service) clear() {
	s.stop()
	s.mu.Lock()
	s.lastErr = ""
	s.mu.Unlock()
}

func (s *Service) persistActive(id string) {
	if err := s.store.SetActive(id); err != nil {
		s.log.Printf("could not persist the active node: %v", err)
	}
}

func find(subs []store.Subscription, nodeID string) (domain.Node, string, bool) {
	for _, sub := range subs {
		for _, n := range sub.Nodes {
			if n.ID == nodeID {
				return n, sub.ID, true
			}
		}
	}
	return domain.Node{}, "", false
}
