// Package connection owns the active VPN session: which node is connected,
// in TUN or proxy mode, and drives an engine.Engine to make it so.
package connection

import (
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"

	"github.com/luynrs/justray/internal/daemon/engine"
	"github.com/luynrs/justray/internal/daemon/platform/elevate"
	"github.com/luynrs/justray/internal/daemon/store"
	"github.com/luynrs/justray/internal/shared/domain"
	"github.com/luynrs/justray/internal/shared/rpc"
)

const (
	Port     = 10808 // TODO: settings ui
	TunIface = "justray"
)

type Status = rpc.Status

type Service struct {
	store     store.Disk
	newEngine engine.New
	log       *log.Logger
	dir       string

	opMu sync.Mutex

	mu      sync.Mutex
	eng     engine.Engine
	node    domain.Node
	sub     string
	started time.Time
	lastErr string
	tun     bool
	tunLive bool
	probes  map[string]engine.Result

	watchers map[chan Status]struct{}
}

func New(dir string, st store.Disk, newEngine engine.New, logger *log.Logger) *Service {
	state, err := st.State()
	if err != nil && logger != nil {
		logger.Printf("could not read state: %v", err)
	}
	return &Service{
		store:     st,
		newEngine: newEngine,
		log:       logger,
		dir:       dir,
		tun:       state.Tun,
		probes:    map[string]engine.Result{},
		watchers:  map[chan Status]struct{}{},
	}
}

func (s *Service) Connect(id string) (Status, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	subs, err := s.store.Subscriptions()
	if err != nil {
		return Status{}, err
	}
	n, sub, ok := find(subs, id)
	if !ok {
		return Status{}, fmt.Errorf("node %q not found", id)
	}

	return s.finish(s.start(n, sub))
}

func (s *Service) Disconnect() (Status, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.Lock()
	name := s.node.Name
	s.mu.Unlock()

	s.clear()

	if name != "" {
		s.log.Printf("disconnected from %s", name)
	}
	return s.finish(nil)
}

func (s *Service) SetTun(enable bool) (Status, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.Lock()
	s.tun = enable
	if err := s.store.SetTun(enable); err != nil {
		s.log.Printf("could not persist tun state: %v", err)
	}
	eng, tunLive := s.eng, s.tunLive
	s.mu.Unlock()

	var err error
	if eng != nil && enable != tunLive {
		if enable {
			err = eng.TunAdd(TunIface)
		} else {
			err = eng.TunRemove(TunIface)
		}
	}

	if err != nil && enable && elevate.Needed(err) {
		go elevate.Tun(s.log, s.dir)
		return s.finish(fmt.Errorf("granting tun permission, reconnecting…"))
	}

	s.mu.Lock()
	if err == nil {
		s.tunLive = enable
	} else {
		s.lastErr = err.Error()
	}
	s.mu.Unlock()

	return s.finish(err)
}

// Shutdown tears the active engine down without broadcasting, for process exit.
func (s *Service) Shutdown() {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.stop()
}

// ActiveID returns the persisted active node id, or "" if none.
func (s *Service) ActiveID() (string, error) {
	return s.store.Active()
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status()
}

func (s *Service) status() Status {
	st := Status{Port: Port, Tun: s.tun, LastErr: s.lastErr}
	if s.eng != nil {
		st.Connected = true
		st.NodeID, st.NodeName = s.node.ID, s.node.Name
		st.Uptime = int64(time.Since(s.started).Seconds())
	}
	return st
}

// finish broadcasts the post-op status and passes err through.
func (s *Service) finish(err error) (Status, error) {
	return s.broadcast(), err
}
