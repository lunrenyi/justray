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

func (s *Service) Restore() {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	defer s.broadcast()
	defer s.arm()

	id, err := s.store.Active()
	if err != nil || id == "" {
		return
	}
	subs, err := s.store.Subscriptions()
	if err != nil {
		s.log.Print(err)
		return
	}
	n, sub, ok := find(subs, id)
	if !ok {
		return
	}
	if err := s.start(n, sub); err != nil {
		s.log.Print(err)
	}
}

// ForgetIfRemoved drops the active node when its subscription is deleted.
func (s *Service) ForgetIfRemoved(subID string, nodes []domain.Node) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	gone := func(id string) bool {
		return id != "" && slices.ContainsFunc(nodes, func(n domain.Node) bool { return n.ID == id })
	}
	if active, err := s.store.Active(); err == nil && gone(active) {
		if err := s.store.SetActive(""); err != nil {
			s.log.Print(err)
		}
	}
	if last, err := s.store.Last(); err == nil && gone(last) {
		if err := s.store.SetLast(""); err != nil {
			s.log.Print(err)
		}
	}

	s.mu.Lock()
	live := s.session.sub == subID
	name := s.session.node.Name
	s.mu.Unlock()
	if !live {
		return
	}

	s.clear()
	if name != "" {
		s.log.Printf("disconnected from %s", name)
	}
	s.broadcast()
}

func (s *Service) start(n domain.Node, sub string) (err error) {
	s.mu.Lock()
	tun, cur := s.tun, s.session
	s.mu.Unlock()

	eng := cur.eng
	if cur.eng != nil && !cur.blocked {
		err = cur.eng.Swap(n)
		if err == nil && tun != cur.tun {
			reconcile := cur.eng.TunRemove
			if tun {
				reconcile = cur.eng.TunAdd
			}
			err = reconcile()
		}
	} else {
		s.stop()

		if err = rpc.ClearLog(rpc.EngineLog(s.dir)); err != nil {
			s.log.Print(err)
		}

		eng = s.newEngine(s.current(), rpc.EngineLog(s.dir))
		if err = eng.Start(n, tun); err != nil {
			_ = eng.Close()
		}
	}
	if err != nil {
		if tun && elevate.Needed(err) {
			s.persistActive(n.ID)
			go elevate.Tun(s.log, s.dir)
			err = errors.New(rpc.ElevateMsg)
		}
		s.setErr(err)
		return err
	}

	s.mu.Lock()
	s.session = session{eng: eng, node: n, sub: sub, started: time.Now(), tun: tun}
	s.lastErr = ""
	s.mu.Unlock()

	s.persistActive(n.ID)
	s.log.Printf("connected to %s (%s %s:%d)", n.Name, n.Protocol, n.Server, n.Port)
	return nil
}

func (s *Service) stop() {
	s.mu.Lock()
	eng := s.session.eng
	s.session = session{}
	s.mu.Unlock()

	if eng == nil {
		return
	}
	if err := eng.Close(); err != nil {
		s.log.Print(err)
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
	s.raise()
}

func (s *Service) arm() {
	s.mu.Lock()
	idle := s.session.eng == nil && s.tun && s.settings.KillSwitch == "on"
	s.mu.Unlock()
	if idle {
		s.raise()
	}
}

func (s *Service) raise() {
	if err := s.block(); err != nil {
		s.log.Print(err)
		s.stop()
		s.setErr(err)
	}
}
func (s *Service) block() error {
	eng := s.newEngine(s.current(), rpc.EngineLog(s.dir))
	if err := eng.Stage(); err != nil {
		_ = eng.Close()
		return err
	}

	s.mu.Lock()
	old := s.session.eng
	s.session = session{eng: eng, tun: true, blocked: true}
	s.mu.Unlock()

	if old != nil {
		if err := old.Close(); err != nil {
			s.log.Print(err)
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
		s.log.Print(err)
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
