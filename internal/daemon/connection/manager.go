package connection

import (
	"errors"
	"slices"
	"time"

	"github.com/luynrs/justray/internal/daemon/platform/elevate"
	"github.com/luynrs/justray/internal/daemon/store"
	"github.com/luynrs/justray/internal/shared/domain"
	"github.com/luynrs/justray/internal/shared/rpc"
)

const elevateMsg = "granting tun permission, reconnecting…"

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

// ForgetIfRemoved drops the active node when its subscription is deleted.
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
	tun := s.tun
	live := s.eng
	hot := live != nil && !s.blocking && s.tunLive == tun
	s.mu.Unlock()

	eng := live
	if hot {
		if err := live.Swap(n); err != nil {
			s.setErr(err)
			return err
		}
	} else {
		s.stop()

		if err := rpc.ClearLog(rpc.EngineLog(s.dir)); err != nil {
			s.log.Printf("could not truncate engine log: %v", err)
		}

		eng = s.newEngine(s.current(), rpc.EngineLog(s.dir))
		if err := eng.Start(n, tun); err != nil {
			s.setErr(err)
			eng.Close()
			if tun && elevate.Needed(err) {
				s.persistActive(n.ID)
				go elevate.Tun(s.log, s.dir)
				return errors.New(elevateMsg)
			}
			return err
		}
	}

	s.mu.Lock()
	s.eng, s.node, s.sub, s.started, s.lastErr, s.tunLive = eng, n, sub, time.Now(), "", tun
	s.mu.Unlock()

	s.persistActive(n.ID)
	s.log.Printf("connected to %s (%s %s:%d)", n.Name, n.Protocol, n.Server, n.Port)
	return nil
}

func (s *Service) stop() {
	s.mu.Lock()
	eng := s.eng
	s.eng, s.node, s.sub, s.tunLive, s.blocking = nil, domain.Node{}, "", false, false
	s.mu.Unlock()

	if eng == nil {
		return
	}
	if err := eng.Close(); err != nil {
		s.log.Printf("closing the engine: %v", err)
	}
}

func (s *Service) clear() {
	s.mu.Lock()
	s.lastErr = ""
	block := s.tun && s.settings.KillSwitch == "on"
	s.mu.Unlock()

	s.persistActive("")
	if !block {
		s.stop()
		return
	}
	if err := s.block(); err != nil {
		s.log.Printf("kill switch: %v", err)
		s.stop()
		s.setErr(err)
	}
}
func (s *Service) block() error {
	eng := s.newEngine(s.current(), rpc.EngineLog(s.dir))
	if err := eng.Stage(); err != nil {
		eng.Close()
		return err
	}

	s.mu.Lock()
	old := s.eng
	s.eng, s.node, s.sub, s.tunLive, s.blocking = eng, domain.Node{}, "", true, true
	s.mu.Unlock()

	if old != nil {
		if err := old.Close(); err != nil {
			s.log.Printf("closing the engine: %v", err)
		}
	}
	return eng.Block()
}

func (s *Service) setErr(err error) {
	s.mu.Lock()
	s.lastErr = err.Error()
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
