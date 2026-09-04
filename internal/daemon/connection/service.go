package connection

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/luynrs/justray/internal/domain"
	"github.com/luynrs/justray/internal/engine"
	"github.com/luynrs/justray/internal/ipc"
	"github.com/luynrs/justray/internal/platform/elevate"
)

type session struct {
	eng     engine.Engine
	node    domain.Node
	ref     domain.NodeRef
	started time.Time
	tun     bool
	port    int
}

type Service struct {
	ctx       context.Context
	newEngine engine.NewFunc
	probeAll  engine.ProbeFunc
	log       *log.Logger
	dir       string

	mu      sync.RWMutex
	session session
	restart chan struct{}
}

func New(ctx context.Context, dir string, newEngine engine.NewFunc, probe engine.ProbeFunc, logger *log.Logger) *Service {
	return &Service{
		ctx:       ctx,
		newEngine: newEngine,
		probeAll:  probe,
		log:       logger,
		dir:       dir,
		restart:   make(chan struct{}, 1),
	}
}

func (s *Service) Connect(ctx context.Context, n domain.Node, ref domain.NodeRef, settings domain.Settings, tun bool) error {
	return s.apply(ctx, n, ref, settings, tun, true)
}

func (s *Service) Apply(ctx context.Context, n domain.Node, ref domain.NodeRef, settings domain.Settings, tun bool) error {
	return s.apply(ctx, n, ref, settings, tun, false)
}

func (s *Service) Disconnect(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.RLock()
	name := s.session.node.Name
	s.mu.RUnlock()
	if err := s.stop(); err != nil {
		return err
	}
	if name != "" {
		s.log.Printf("disconnected from %s", name)
	}
	return nil
}

func (s *Service) Restore(n domain.Node, ref domain.NodeRef, settings domain.Settings, tun bool) {
	if err := s.apply(s.ctx, n, ref, settings, tun, true); err != nil {
		s.log.Print(err)
	}
}

func (s *Service) ForgetIfRemoved(subID string) error {
	s.mu.RLock()
	sub := s.session.ref.SubscriptionID
	s.mu.RUnlock()
	if sub != subID {
		return nil
	}
	if err := s.Disconnect(context.Background()); err != nil {
		s.log.Print(err)
		return err
	}
	return nil
}

func (s *Service) Probe(ctx context.Context, nodes []domain.Node, settings domain.Settings, onResult func(string, engine.Result)) (map[string]engine.Result, error) {
	return s.probeAll(ctx, nodes, settings, ipc.EngineLog(s.dir), onResult)
}

func (s *Service) RestartRequested() <-chan struct{} { return s.restart }

func (s *Service) Shutdown() {
	if err := s.stop(); err != nil {
		s.log.Print(err)
	}
}

func (s *Service) Status() ipc.Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := ipc.Status{}
	if s.session.eng != nil && s.session.eng.Running() {
		st.Connected = true
		st.NodeRef, st.NodeName = s.session.ref, s.session.node.Name
		st.Uptime = int64(time.Since(s.session.started).Seconds())
		st.Tun = s.session.tun
		st.Port = s.session.port
	}
	return st
}

func (s *Service) apply(ctx context.Context, n domain.Node, ref domain.NodeRef, settings domain.Settings, tun, resetStarted bool) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if n.TLS != nil && n.TLS.Insecure {
		return errors.New("insecure TLS node is not allowed")
	}
	s.mu.RLock()
	eng := s.session.eng
	started := s.session.started
	s.mu.RUnlock()
	if eng == nil {
		if err = ipc.ClearLog(ipc.EngineLog(s.dir)); err != nil {
			s.log.Print(err)
		}
		eng = s.newEngine(s.ctx, ipc.EngineLog(s.dir))
		if eng == nil {
			err = errors.New("initialize engine: engine is nil")
		} else if err = eng.Apply(ctx, engine.SessionSpec{Node: n, Settings: settings, Tun: tun}); err != nil {
			err = errors.Join(err, eng.Stop())
		}
	} else {
		err = eng.Apply(ctx, engine.SessionSpec{Node: n, Settings: settings, Tun: tun})
	}
	if err != nil {
		if eng != nil && !eng.Running() {
			s.mu.Lock()
			s.session = session{}
			s.mu.Unlock()
		}
		if tun && elevate.Needed(err) {
			select {
			case s.restart <- struct{}{}:
			default:
			}
			err = ipc.ErrElevate
		}
		return err
	}

	if resetStarted || started.IsZero() {
		started = time.Now()
	}
	s.mu.Lock()
	s.session = session{eng: eng, node: n, ref: ref, started: started, tun: tun, port: settings.Port}
	s.mu.Unlock()
	s.log.Printf("connected to %s (%s %s:%d)", n.Name, n.Protocol, n.Server, n.Port)
	return nil
}

func (s *Service) stop() error {
	s.mu.Lock()
	eng := s.session.eng
	s.session = session{}
	s.mu.Unlock()
	if eng != nil {
		return eng.Stop()
	}
	return nil
}
