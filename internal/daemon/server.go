package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/luynrs/justxray/internal/daemon/runner"
	"github.com/luynrs/justxray/internal/daemon/store"
)

// has to cover a full Probe of every node
const idle = 60 * time.Second

type Server struct {
	dir    string
	store  store.Disk
	runner *runner.Process
	log    *log.Logger
	device http.Header

	mu       sync.Mutex
	sub      string // subscription the active node belongs to
	probes   map[string]probeResult
	watchers map[chan Status]struct{}
}

func New(dir string, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	device, err := deviceHeaders()
	if err != nil {
		logger.Printf("device: %v, subscriptions needing a device id won't resolve", err)
	}
	return &Server{
		dir:      dir,
		store:    store.Disk{Dir: dir},
		runner:   runner.New(),
		log:      logger,
		device:   device,
		probes:   map[string]probeResult{},
		watchers: map[chan Status]struct{}{},
	}
}

func Listen(socket string) (net.Listener, error) {
	if conn, err := net.DialTimeout("unix", socket, time.Second); err == nil {
		conn.Close()
		return nil, fmt.Errorf("another justxrayd is already listening on %s", socket)
	}
	os.Remove(socket)

	ln, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

func (s *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handle(conn)
	}
}

func (s *Server) Shutdown() { s.runner.Stop() }

// reconnect to the last active node
func (s *Server) Restore() {
	s.mu.Lock()
	defer s.mu.Unlock()

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

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(idle))

	var req Req
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		reply(conn, nil, fmt.Errorf("bad request: %w", err))
		return
	}
	if req.Method == "Watch" {
		s.watch(conn)
		return
	}
	result, err := s.dispatch(req)
	reply(conn, result, err)
}

func (s *Server) dispatch(req Req) (any, error) {
	a := req.Args
	switch req.Method {
	case "Ping":
		return "pong", nil
	case "Subs":
		return s.subs()
	case "AddSub":
		return s.addSub(a.URL)
	case "RemoveSub":
		return nil, s.removeSub(a.ID)
	case "RefreshAll":
		return s.refreshAll()
	case "Refresh":
		return s.refresh(a.ID)
	case "Nodes":
		return s.nodes()
	case "Probe":
		return s.probe(a.Sub, a.ID)
	case "Connect":
		return s.connect(a.ID)
	case "Disconnect":
		return s.disconnect()
	}
	return nil, fmt.Errorf("unknown method %q", req.Method)
}

func (s *Server) watch(conn net.Conn) {
	conn.SetDeadline(time.Time{}) // stays open

	ch := make(chan Status, 1)
	s.mu.Lock()
	s.watchers[ch] = struct{}{}
	initial := s.status()
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.watchers, ch)
		s.mu.Unlock()
	}()

	gone := make(chan struct{})
	go func() {
		conn.Read(make([]byte, 1)) // blocks until the client disconnects
		close(gone)
	}()

	enc := json.NewEncoder(conn)
	if err := enc.Encode(initial); err != nil {
		return
	}
	for {
		select {
		case <-gone:
			return
		case st := <-ch:
			if err := enc.Encode(st); err != nil {
				return
			}
		}
	}
}

func (s *Server) broadcast() {
	st := s.status()
	for ch := range s.watchers {
		select {
		case ch <- st:
		default:
		}
	}
}

func reply(conn net.Conn, result any, err error) {
	if err != nil {
		json.NewEncoder(conn).Encode(Resp{Error: err.Error()})
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		json.NewEncoder(conn).Encode(Resp{Error: err.Error()})
		return
	}
	json.NewEncoder(conn).Encode(Resp{OK: true, Result: raw})
}
