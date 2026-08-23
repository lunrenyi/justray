// Package connection owns the active session and drives an engine.Engine
package connection

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"

	"github.com/luynrs/justray/internal/daemon/engine"
	"github.com/luynrs/justray/internal/daemon/platform/autostart"
	"github.com/luynrs/justray/internal/daemon/platform/elevate"
	"github.com/luynrs/justray/internal/daemon/store"
	"github.com/luynrs/justray/internal/shared/domain"
	"github.com/luynrs/justray/internal/shared/rpc"
)

type Status = rpc.Status

type Service struct {
	store     store.Disk
	newEngine engine.New
	probeAll  engine.Probe
	log       *log.Logger
	dir       string

	opMu sync.Mutex

	mu       sync.Mutex
	eng      engine.Engine
	node     domain.Node
	sub      string
	started  time.Time
	lastErr  string
	tun      bool
	tunLive  bool
	blocking bool
	settings domain.Settings
	probes   map[string]engine.Result

	watchers map[chan Status]struct{}
}

func New(dir string, st store.Disk, newEngine engine.New, probe engine.Probe, logger *log.Logger) *Service {
	state, err := st.State()
	if err != nil && logger != nil {
		logger.Printf("could not read state: %v", err)
	}
	settings, err := state.Settings.Normalize()
	if err != nil && logger != nil {
		logger.Printf("stored settings rejected, falling back to defaults: %v", err)
		settings, _ = domain.Settings{}.Normalize()
	}
	return &Service{
		store:     st,
		newEngine: newEngine,
		probeAll:  probe,
		log:       logger,
		dir:       dir,
		tun:       state.Tun,
		settings:  settings,
		probes:    map[string]engine.Result{},
		watchers:  map[chan Status]struct{}{},
	}
}

// Settings is for the client: it reads the autostart the OS actually has.
func (s *Service) Settings() domain.Settings {
	out := s.current()
	out.Autostart = toggle(autostart.Enabled())
	return out
}

func (s *Service) current() domain.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// RefreshEvery is the subscription refresh interval in hours, 0 when off.
func (s *Service) RefreshEvery() int { return s.current().RefreshEvery }

func toggle(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func (s *Service) SetSettings(in domain.Settings) (Status, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	in, err := in.Normalize()
	if err != nil {
		return s.Status(), err
	}

	// the OS comes first: if it refuses, nothing is persisted anywhere
	if in.Autostart != toggle(autostart.Enabled()) {
		apply := autostart.Enable
		if in.Autostart == "off" {
			apply = autostart.Disable
		}
		if err := apply(); err != nil {
			s.log.Printf("autostart: %v", err)
			return s.Status(), err
		}
	}
	if err := s.store.SetSettings(in); err != nil {
		return s.Status(), err
	}

	s.mu.Lock()
	old := s.settings
	s.settings = in
	blocked := s.blocking
	n, sub := s.node, s.sub
	live := s.eng != nil && !blocked
	s.mu.Unlock()

	if blocked {
		switch {
		case in.KillSwitch != "on":
			s.stop()
		case engineChanged(old, in):
			s.clear()
		}
		return s.finish(nil)
	}

	if !live || !engineChanged(old, in) {
		return s.finish(nil)
	}
	s.stop()
	return s.finish(s.start(n, sub))
}

func engineChanged(x, y domain.Settings) bool {
	x.ProbeURL, y.ProbeURL = "", ""
	x.RefreshEvery, y.RefreshEvery = 0, 0
	x.KillSwitch, y.KillSwitch = "", ""
	x.Autostart, y.Autostart = "", ""
	return !x.Equal(y)
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
	eng, tunLive, blocking := s.eng, s.tunLive, s.blocking
	s.mu.Unlock()

	// the kill switch is only a TUN; turning TUN off takes it down with it
	if blocking {
		if !enable {
			s.stop()
		}
		return s.finish(nil)
	}

	var err error
	if eng != nil && enable != tunLive {
		if enable {
			err = eng.TunAdd()
		} else {
			err = eng.TunRemove()
		}
	}

	if err != nil && enable && elevate.Needed(err) {
		go elevate.Tun(s.log, s.dir)
		return s.finish(errors.New(elevateMsg))
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
	st := Status{Port: s.settings.Port, Tun: s.tun, LastErr: s.lastErr, Blocked: s.blocking}
	if s.eng != nil && !s.blocking {
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
