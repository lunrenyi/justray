package runner

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/luynrs/justxray/internal/daemon/procgroup"
)

type Process struct {
	bin    string
	log    string
	onExit func() // fires on any exit, crash or Stop

	mu       sync.Mutex
	cmd      *exec.Cmd
	done     chan struct{}
	started  time.Time
	id       string
	name     string
	stopping bool
	lastErr  string
}

type Status struct {
	Running  bool
	PID      int
	NodeID   string
	NodeName string
	Uptime   time.Duration
	LastErr  string
}

func New(bin, log string, onExit func()) *Process {
	return &Process{bin: bin, log: log, onExit: onExit}
}

func (r *Process) Start(config, nodeID, nodeName string) error {
	r.Stop()

	r.mu.Lock()
	defer r.mu.Unlock()

	logFile, err := os.OpenFile(r.log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", r.log, err)
	}

	cmd := exec.Command(r.bin, "run", "-c", config)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	procgroup.Setup(cmd)

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("%s: %w", filepath.Base(r.bin), err)
	}

	done := make(chan struct{})
	r.cmd, r.done, r.started = cmd, done, time.Now()
	r.id, r.name, r.lastErr, r.stopping = nodeID, nodeName, "", false

	go func() {
		err := cmd.Wait()
		logFile.Close()

		r.mu.Lock()
		if r.cmd == cmd {
			r.cmd, r.id, r.name, r.lastErr = nil, "", "", ""
			if err != nil && !r.stopping {
				r.lastErr = cmp.Or(lastLine(r.log), err.Error())
			}
		}
		r.mu.Unlock()

		close(done)
		if r.onExit != nil {
			r.onExit()
		}
	}()
	return nil
}

// SIGTERM, then SIGKILL after 5s
func (r *Process) Stop() {
	r.mu.Lock()
	cmd, done := r.cmd, r.done
	r.stopping = true
	r.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}
	procgroup.Terminate(cmd)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		procgroup.Kill(cmd)
		<-done
	}
}

func (r *Process) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return Status{LastErr: r.lastErr}
	}
	return Status{
		Running:  true,
		PID:      r.cmd.Process.Pid,
		NodeID:   r.id,
		NodeName: r.name,
		Uptime:   time.Since(r.started),
	}
}

func lastLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	end, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return ""
	}
	const window = 4 << 10
	buf := make([]byte, min(end, window))
	if _, err := f.ReadAt(buf, end-int64(len(buf))); err != nil {
		return ""
	}

	line := strings.TrimSpace(string(buf))
	if i := strings.LastIndexByte(line, '\n'); i >= 0 {
		line = line[i+1:]
	}
	if date, rest, ok := strings.Cut(line, " "); ok && strings.Count(date, "/") == 2 {
		if _, after, ok := strings.Cut(rest, " "); ok {
			line = after
		}
	}
	if len(line) > 200 {
		line = line[:200] + "…"
	}
	return line
}
