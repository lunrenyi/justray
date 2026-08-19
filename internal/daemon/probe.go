package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	sbox "github.com/sagernet/sing-box"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/luynrs/justray/internal/daemon/core"
	"github.com/luynrs/justray/internal/parser/proxy"
)

const (
	probeURL     = "http://cp.cloudflare.com/generate_204"
	probeTimeout = 5 * time.Second
	probeWorkers = 12
)

type probeResult struct {
	alive bool
	ms    int
}

func (s *Server) probeNodes(nodes []proxy.Node) (map[string]probeResult, error) {
	inst, err := sbox.New(sbox.Options{Options: *core.ProbeConfig(nodes, coreLog(s.dir)), Context: core.Context(context.Background())})
	if err != nil {
		return nil, fmt.Errorf("build probe engine: %w", err)
	}
	if err := inst.Start(); err != nil {
		inst.Close()
		return nil, fmt.Errorf("start probe engine: %w", err)
	}
	defer inst.Close()

	out := map[string]probeResult{}
	sem := make(chan struct{}, probeWorkers)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, n := range nodes {
		dialer, ok := inst.Outbound().Outbound(core.ProbeTag(i))
		if !ok {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			ms, err := delay(dialer)
			mu.Lock()
			out[n.ID] = probeResult{alive: err == nil, ms: ms}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out, nil
}

func delay(dialer N.Dialer) (int, error) {
	client := &http.Client{
		Timeout: probeTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddr(addr))
			},
		},
	}
	defer client.CloseIdleConnections()

	start := time.Now()
	resp, err := client.Get(probeURL)
	ms := int(time.Since(start).Milliseconds())
	if err != nil {
		return ms, err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ms, fmt.Errorf("http %d", resp.StatusCode)
	}
	return ms, nil
}
