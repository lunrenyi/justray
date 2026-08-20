package daemon

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"syscall"
	"time"

	"github.com/sagernet/netlink"
	sbox "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service"

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
	s.opMu.Lock()
	defer s.opMu.Unlock()

	subs, err := s.subscriptions()
	if err != nil {
		return Status{}, err
	}
	n, sub, ok := find(subs, id)
	if !ok {
		return Status{}, fmt.Errorf("node %q not found", id)
	}

	return s.finish(s.start(n, sub))
}

func (s *Server) disconnect() (Status, error) {
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

// finish broadcasts the post-op status and passes err through.
func (s *Server) finish(err error) (Status, error) {
	return s.broadcast(), err
}

func (s *Server) stop() {
	s.mu.Lock()
	inst, tunLive := s.inst, s.tunLive
	s.inst, s.node, s.sub, s.tunLive = nil, proxy.Node{}, "", false
	s.mu.Unlock()

	if inst == nil {
		return
	}
	if err := inst.Close(); err != nil {
		s.log.Printf("closing the engine: %v", err)
	}
	if tunLive {
		waitGone(tunInterface)
	}
}

func newEngine(opts option.Options) (*sbox.Box, error) {
	inst, err := sbox.New(sbox.Options{Options: opts, Context: core.Context(context.Background())})
	if err == nil {
		err = inst.Start()
	}
	return inst, err
}

func waitGone(iface string) bool {
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if _, err := net.InterfaceByName(iface); err != nil {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func forceDeleteLink(iface string) {
	if link, err := netlink.LinkByName(iface); err == nil {
		netlink.LinkDel(link)
	}
}

func (s *Server) clear() {
	s.stop()
	s.mu.Lock()
	s.lastErr = ""
	s.mu.Unlock()
	if err := s.store.SetActive(""); err != nil {
		s.log.Printf("could not clear the active node: %v", err)
	}
}

func (s *Server) start(n proxy.Node, sub string) error {
	s.mu.Lock()
	iface := ""
	if s.tun {
		iface = tunInterface
	}
	s.mu.Unlock()

	s.stop()

	opts, err := core.Build(n, port, coreLog(s.dir), iface)
	var inst *sbox.Box
	if err == nil {
		inst, err = newEngine(*opts)
	}
	for i := 0; i < 2 && err != nil && iface != "" && errors.Is(err, syscall.EBUSY); i++ {
		if inst != nil {
			inst.Close()
		}
		waitGone(iface)
		inst, err = newEngine(*opts)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.lastErr = err.Error()
		if inst != nil {
			inst.Close()
		}
		if iface != "" && elevate.Needed(err) {
			if err := s.store.SetActive(n.ID); err != nil {
				s.log.Printf("could not persist the active node: %v", err)
			}
			go elevate.Tun(s.log, s.dir)
			return fmt.Errorf("granting tun permission, reconnecting…")
		}
		return err
	}

	s.inst, s.node, s.sub, s.started, s.lastErr, s.tunLive = inst, n, sub, time.Now(), "", iface != ""
	if err := s.store.SetActive(n.ID); err != nil {
		s.log.Printf("could not persist the active node: %v", err)
	}
	s.log.Printf("connected to %s (%s %s:%d)", n.Name, n.Protocol, n.Server, n.Port)
	return nil
}

func (s *Server) setTun(enable bool) (Status, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.Lock()
	s.tun = enable
	if err := s.store.SetTun(enable); err != nil {
		s.log.Printf("could not persist tun state: %v", err)
	}
	inst, tunLive := s.inst, s.tunLive
	s.mu.Unlock()

	var err error
	if inst != nil && enable != tunLive {
		if enable {
			err = s.tunAdd(inst)
		} else {
			err = s.tunRemove(inst)
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

func (s *Server) tunAdd(inst *sbox.Box) error {
	inb := core.TunInbound(tunInterface, core.Resolvers())
	ctx := service.ContextWith[adapter.NetworkManager](core.Context(context.Background()), inst.Network())
	logger := inst.LogFactory().NewLogger("inbound/tun[tun-in]")

	var err error
	for i := 0; i < 3; i++ {
		err = inst.Inbound().Create(ctx, inst.Router(), logger, "tun-in", C.TypeTun, inb.Options)
		if err == nil || !errors.Is(err, syscall.EBUSY) {
			return err
		}
		forceDeleteLink(tunInterface)
		waitGone(tunInterface)
	}
	return err
}

func (s *Server) tunRemove(inst *sbox.Box) error {
	err := inst.Inbound().Remove("tun-in")
	if waitGone(tunInterface) {
		return err
	}
	forceDeleteLink(tunInterface)
	if !waitGone(tunInterface) {
		return fmt.Errorf("%s still up after removing tun-in", tunInterface)
	}
	return err
}

func (s *Server) status() Status {
	st := Status{Port: port, Tun: s.tun, LastErr: s.lastErr}
	if s.inst != nil {
		st.Connected = true
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
				Sub: sub.ID,
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
	subs, err := s.subscriptions()
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
