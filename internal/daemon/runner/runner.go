package runner

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/xtls/xray-core/core"
)

type Process struct {
	mu      sync.Mutex
	inst    *core.Instance
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

func (r *Process) Start(config []byte, nodeID, nodeName string) error {
	r.Stop()

	r.mu.Lock()
	defer r.mu.Unlock()

	inst, err := core.StartInstance("json", config)
	if err != nil {
		r.lastErr = err.Error()
		return fmt.Errorf("start xray: %w", err)
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
