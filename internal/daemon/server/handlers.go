package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/luynrs/justray/internal/shared/rpc"
)

const maxRequestSize = 1 << 20

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(idle))

	var req rpc.Req
	if err := json.NewDecoder(io.LimitReader(conn, maxRequestSize)).Decode(&req); err != nil {
		reply(conn, nil, fmt.Errorf("bad request: %w", err))
		return
	}
	if req.Method == "Watch" {
		s.watch(conn)
		return
	}
	conn.SetDeadline(time.Time{})
	result, err := s.dispatch(req)
	conn.SetDeadline(time.Now().Add(idle))
	reply(conn, result, err)
}

func (s *Server) dispatch(req rpc.Req) (any, error) {
	a := req.Args
	switch req.Method {
	case "Ping":
		return "pong", nil
	case "Status":
		return s.conn.Status(), nil
	case "Active":
		return s.conn.ActiveID()
	case "Subs":
		return s.subs.List()
	case "AddSub":
		return s.subs.Add(a.URL)
	case "RemoveSub":
		return nil, s.removeSub(a.ID)
	case "RefreshAll":
		return s.subs.RefreshAll()
	case "Refresh":
		return s.subs.Refresh(a.ID)
	case "Nodes":
		return s.conn.Nodes()
	case "Probe":
		return s.conn.Probe(a.Sub, a.ID)
	case "Connect":
		return s.conn.Connect(a.ID)
	case "Disconnect":
		return s.conn.Disconnect()
	case "SetTun":
		return s.conn.SetTun(a.Tun)
	}
	return nil, fmt.Errorf("unknown method %q", req.Method)
}

// removeSub deletes the subscription and, if it's the one currently
// connected, drops the connection along with it.
func (s *Server) removeSub(id string) error {
	sub, err := s.subs.Remove(id)
	if err != nil {
		return err
	}
	s.conn.ForgetIfRemoved(sub.ID, sub.Nodes)
	return nil
}

func (s *Server) watch(conn net.Conn) {
	conn.SetDeadline(time.Time{}) // stays open

	initial, ch, cancel := s.conn.Watch()
	defer cancel()

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

func reply(conn net.Conn, result any, err error) {
	if err != nil {
		json.NewEncoder(conn).Encode(rpc.Resp{Error: err.Error()})
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		json.NewEncoder(conn).Encode(rpc.Resp{Error: err.Error()})
		return
	}
	json.NewEncoder(conn).Encode(rpc.Resp{OK: true, Result: raw})
}
