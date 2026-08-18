package runner

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	sbox "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"

	"github.com/luynrs/justray/internal/daemon/core"
)

type Process struct {
	mu      sync.Mutex
	inst    *sbox.Box
	started time.Time
	id      string
	name    string
	lastErr string
}

type Status struct {
	Running  bool
	PID      int
	NodeID   string
	NodeName string
	Uptime   time.Duration
	LastErr  string
}

func New() *Process { return &Process{} }

func (r *Process) Start(opts *option.Options, nodeID, nodeName string) error {
	r.Stop()

	r.mu.Lock()
	defer r.mu.Unlock()

	inst, err := sbox.New(sbox.Options{Options: *opts, Context: core.Context(context.Background())})
	if err != nil {
		r.lastErr = err.Error()
		return fmt.Errorf("build engine: %w", err)
	}
	if err := inst.Start(); err != nil {
		r.lastErr = err.Error()
		return fmt.Errorf("start engine: %w", err)
	}

	r.inst, r.started = inst, time.Now()
	r.id, r.name, r.lastErr = nodeID, nodeName, ""
	return nil
}

func (r *Process) Stop() {
	r.mu.Lock()
	inst := r.inst
	r.inst, r.id, r.name = nil, "", ""
	r.mu.Unlock()

	if inst == nil {
		return
	}
	inst.Close()
}

func (r *Process) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.inst == nil {
		return Status{LastErr: r.lastErr}
	}
	return Status{
		Running:  true,
		PID:      os.Getpid(),
		NodeID:   r.id,
		NodeName: r.name,
		Uptime:   time.Since(r.started),
	}
}
